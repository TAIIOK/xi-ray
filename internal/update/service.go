package update

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/taiiok/xiaomi-vless/internal/config"
	"github.com/taiiok/xiaomi-vless/internal/version"
)

type StatusResponse struct {
	CurrentVersion  string       `json:"current_version"`
	CurrentCommit   string       `json:"current_commit"`
	PreviousVersion string       `json:"previous_version,omitempty"`
	Phase           Phase        `json:"phase"`
	TargetVersion   string       `json:"target_version,omitempty"`
	Progress        float64      `json:"progress,omitempty"`
	Error           string       `json:"error,omitempty"`
	CanRollback     bool         `json:"can_rollback"`
	CanApply        bool         `json:"can_apply"`
	CanDownload     bool         `json:"can_download"`
	Available       *ReleaseInfo `json:"available,omitempty"`
}

type Service struct {
	layout Layout
	store  *StateStore
	client *http.Client
	cfg    func() config.PanelConfig
}

func NewService(home, configPath string, cfg func() config.PanelConfig) *Service {
	layout := LayoutForHome(home, configPath)
	return &Service{
		layout: layout,
		store:  NewStateStore(layout.StatePath),
		client: &http.Client{Timeout: 15 * time.Minute},
		cfg:    cfg,
	}
}

func (s *Service) Layout() Layout { return s.layout }

func (s *Service) Status(ctx context.Context) (StatusResponse, error) {
	st, err := s.store.Load()
	if err != nil {
		return StatusResponse{}, err
	}
	resp := StatusResponse{
		CurrentVersion: version.Version,
		CurrentCommit:  version.Commit,
		Phase:          st.Phase,
		TargetVersion:  st.TargetVersion,
		Error:          st.Error,
		CanRollback:    s.layout.HasPrevious(),
		CanApply:       st.Phase == PhaseVerified,
		CanDownload:    st.Phase == PhaseIdle || st.Phase == PhaseFailed || st.Phase == PhaseRolledBack || st.Phase == PhaseDownloading,
	}
	if st.TotalBytes > 0 {
		resp.Progress = float64(st.DownloadedBytes) / float64(st.TotalBytes) * 100
	}
	if st.PreviousVersion != "" {
		resp.PreviousVersion = st.PreviousVersion
	} else if s.layout.HasPrevious() {
		resp.PreviousVersion = "previous"
	}
	return resp, nil
}

func (s *Service) Check(ctx context.Context) (StatusResponse, error) {
	if err := s.layout.EnsureDirs(); err != nil {
		return StatusResponse{}, err
	}
	st, err := s.store.Load()
	if err != nil {
		return StatusResponse{}, err
	}
	if st.Phase.IsActive() && st.Phase != PhaseDownloading {
		return s.Status(ctx)
	}

	rel, err := FetchLatestRelease(s.client)
	if err != nil {
		return StatusResponse{}, err
	}

	st.Phase = PhaseChecking
	st.TargetVersion = rel.Version
	st.DownloadURL = rel.DownloadURL
	st.PreviousVersion = version.Version
	st.Attempts = 0
	if err := s.store.Save(st); err != nil {
		return StatusResponse{}, err
	}

	resp, err := s.Status(ctx)
	if err != nil {
		return StatusResponse{}, err
	}
	resp.Available = &rel
	if trimVersion(rel.Version) == trimVersion(version.Version) {
		st.Phase = PhaseIdle
		_ = s.store.Save(st)
		resp.Phase = PhaseIdle
	}
	return resp, nil
}

func (s *Service) Download(ctx context.Context, targetVersion string) (StatusResponse, error) {
	if err := s.layout.EnsureDirs(); err != nil {
		return StatusResponse{}, err
	}
	st, err := s.store.Load()
	if err != nil {
		return StatusResponse{}, err
	}

	if st.DownloadURL == "" || (targetVersion != "" && trimVersion(targetVersion) != trimVersion(st.TargetVersion)) {
		rel, err := FetchLatestRelease(s.client)
		if err != nil {
			return StatusResponse{}, err
		}
		if targetVersion != "" && trimVersion(targetVersion) != trimVersion(rel.Version) {
			return StatusResponse{}, fmt.Errorf("version %s not found in latest release", targetVersion)
		}
		st.TargetVersion = rel.Version
		st.DownloadURL = rel.DownloadURL
		st.PreviousVersion = version.Version
	}

	if trimVersion(st.TargetVersion) == trimVersion(version.Version) {
		return StatusResponse{}, fmt.Errorf("already running version %s", version.Version)
	}

	st.Phase = PhaseDownloading
	st.ArchivePath = ArchivePath(s.layout.DownloadsDir, st.TargetVersion)
	st.Attempts++
	if err := s.store.Save(st); err != nil {
		return StatusResponse{}, err
	}

	var dlErr error
	for attempt := 0; attempt < maxDownloadAttempts; attempt++ {
		st, dlErr = DownloadArchive(ctx, s.client, st.DownloadURL, st.ArchivePath, st, func(p State) {
			_ = s.store.Save(p)
		})
		if dlErr == nil {
			break
		}
		st.Attempts++
		_ = s.store.Save(st)
	}
	if dlErr != nil {
		_ = s.store.Fail(st, PhaseFailed, dlErr)
		return s.Status(ctx)
	}

	sumPath := st.ArchivePath + ".sha256"
	if data, err := os.ReadFile(sumPath); err == nil {
		want := strings.Fields(string(data))[0]
		if err := VerifyArchiveChecksum(st.ArchivePath, want); err != nil {
			CleanupPartialDownload(st.ArchivePath)
			_ = s.store.Fail(st, PhaseFailed, err)
			return s.Status(ctx)
		}
	}

	st.Phase = PhaseExtracting
	_ = s.store.Save(st)
	if err := ExtractArchive(st.ArchivePath, s.layout.StagingDir); err != nil {
		_ = s.store.Fail(st, PhaseFailed, err)
		return s.Status(ctx)
	}

	manifest, err := LoadManifest(filepath.Join(s.layout.StagingDir, "manifest.json"))
	if err != nil {
		_ = s.store.Fail(st, PhaseFailed, err)
		return s.Status(ctx)
	}
	if err := manifest.ValidateStaging(s.layout.StagingDir); err != nil {
		_ = s.store.Fail(st, PhaseFailed, err)
		return s.Status(ctx)
	}
	if manifest.MinConfigVersion > config.CurrentVersion {
		err := fmt.Errorf("update requires config version %d (current schema %d)", manifest.MinConfigVersion, config.CurrentVersion)
		_ = s.store.Fail(st, PhaseFailed, err)
		return s.Status(ctx)
	}

	st.TargetVersion = manifest.Version
	st.Phase = PhaseVerified
	st.Checksum = manifest.Assets["panel"].SHA256
	if err := s.store.Save(st); err != nil {
		return StatusResponse{}, err
	}
	return s.Status(ctx)
}

func (s *Service) Cancel(ctx context.Context) (StatusResponse, error) {
	st, err := s.store.Load()
	if err != nil {
		return StatusResponse{}, err
	}
	if st.ArchivePath != "" {
		CleanupPartialDownload(st.ArchivePath)
		CleanupPartialDownload(st.ArchivePath + ".sha256")
	}
	_ = os.RemoveAll(s.layout.StagingDir)
	if err := s.store.Reset(); err != nil {
		return StatusResponse{}, err
	}
	return s.Status(ctx)
}

func (s *Service) Apply(ctx context.Context) (StatusResponse, error) {
	st, err := s.store.Load()
	if err != nil {
		return StatusResponse{}, err
	}
	if st.Phase != PhaseVerified {
		return StatusResponse{}, fmt.Errorf("cannot apply in phase %s", st.Phase)
	}
	if _, err := os.Stat(s.layout.UpdaterScript); err != nil {
		return StatusResponse{}, fmt.Errorf("updater script missing: %w", err)
	}

	st.Phase = PhaseApplying
	if err := s.store.Save(st); err != nil {
		return StatusResponse{}, err
	}

	cmd := exec.CommandContext(ctx, "nohup", s.layout.UpdaterScript, "apply")
	cmd.Dir = s.layout.Home
	if err := cmd.Start(); err != nil {
		_ = s.store.Fail(st, PhaseFailed, err)
		return s.Status(ctx)
	}
	return s.Status(ctx)
}

func (s *Service) Rollback(ctx context.Context) (StatusResponse, error) {
	if !s.layout.HasPrevious() {
		return StatusResponse{}, fmt.Errorf("no previous version to rollback")
	}
	if _, err := os.Stat(s.layout.UpdaterScript); err != nil {
		return StatusResponse{}, fmt.Errorf("updater script missing: %w", err)
	}
	cmd := exec.CommandContext(ctx, s.layout.UpdaterScript, "rollback")
	cmd.Dir = s.layout.Home
	if err := cmd.Run(); err != nil {
		return StatusResponse{}, err
	}
	return s.Status(ctx)
}

func (s *Service) Confirm(ctx context.Context) (StatusResponse, error) {
	st, err := s.store.Load()
	if err != nil {
		return StatusResponse{}, err
	}
	if st.Phase != PhaseHealthCheck && st.Phase != PhaseCompleted {
		return StatusResponse{}, fmt.Errorf("nothing to confirm in phase %s", st.Phase)
	}
	st.Phase = PhaseCompleted
	if err := s.store.Save(st); err != nil {
		return StatusResponse{}, err
	}
	_ = os.RemoveAll(s.layout.StagingDir)
	if st.ArchivePath != "" {
		_ = os.Remove(st.ArchivePath)
		_ = os.Remove(st.ArchivePath + ".sha256")
	}
	if err := s.store.Reset(); err != nil {
		return StatusResponse{}, err
	}
	return s.Status(ctx)
}

func (s *Service) ResumeOrVerify(ctx context.Context) error {
	if err := s.layout.EnsureDirs(); err != nil {
		return err
	}
	st, err := s.store.Load()
	if err != nil {
		return err
	}

	switch st.Phase {
	case PhaseIdle, PhaseCompleted, PhaseRolledBack:
		return nil
	case PhaseDownloading:
		_, err := s.Download(ctx, st.TargetVersion)
		return err
	case PhaseExtracting, PhaseVerified, PhaseApplying, PhaseRestarting:
		if _, err := os.Stat(s.layout.UpdaterScript); err == nil {
			cmd := exec.CommandContext(ctx, s.layout.UpdaterScript, "resume")
			cmd.Dir = s.layout.Home
			_ = cmd.Run()
		}
		return nil
	case PhaseHealthCheck:
		return s.runHealthCheck(ctx)
	case PhaseFailed:
		return nil
	case PhaseChecking:
		st.Phase = PhaseIdle
		return s.store.Save(st)
	default:
		return nil
	}
}

func (s *Service) runHealthCheck(ctx context.Context) error {
	st, err := s.store.Load()
	if err != nil {
		return err
	}

	checkCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	cfg := s.cfg()
	result := RunHealthChecks(checkCtx, cfg, s.layout)
	if result.OK {
		st.Phase = PhaseCompleted
		if err := s.store.Save(st); err != nil {
			return err
		}
		_ = os.RemoveAll(s.layout.StagingDir)
		return s.store.Reset()
	}

	_ = s.store.Fail(st, PhaseHealthCheck, fmt.Errorf("%s", result.Message))
	if _, err := os.Stat(s.layout.UpdaterScript); err == nil {
		cmd := exec.CommandContext(context.Background(), s.layout.UpdaterScript, "rollback")
		cmd.Dir = s.layout.Home
		_ = cmd.Run()
	}
	return fmt.Errorf("health check failed: %s", result.Message)
}

func (s *Service) StartHealthCheckIfNeeded(ctx context.Context) {
	st, err := s.store.Load()
	if err != nil {
		return
	}
	if st.Phase == PhaseRestarting {
		st.Phase = PhaseHealthCheck
		_ = s.store.Save(st)
		go func() {
			time.Sleep(2 * time.Second)
			_ = s.runHealthCheck(context.Background())
		}()
	}
}
