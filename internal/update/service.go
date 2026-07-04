package update

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/taiiok/xiaomi-vless/internal/config"
	"github.com/taiiok/xiaomi-vless/internal/version"
)

type StatusResponse struct {
	CurrentVersion  string       `json:"current_version"`
	CurrentCommit   string       `json:"current_commit"`
	PreviousVersion string       `json:"previous_version,omitempty"`
	RollbackVersion string       `json:"rollback_version,omitempty"`
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
	layout         Layout
	store          *StateStore
	checkClient    *http.Client
	downloadClient *http.Client
	cfg            func() config.PanelConfig
	PostUpdateHook func(context.Context) error
	staleResumeMu  sync.Mutex
	lastStaleResume time.Time
}

func NewService(home, configPath string, cfg func() config.PanelConfig) *Service {
	layout := LayoutForHome(home, configPath)
	return &Service{
		layout:         layout,
		store:          NewStateStore(layout.StatePath),
		checkClient:    CheckHTTPClient(),
		downloadClient: DownloadHTTPClient(),
		cfg:            cfg,
	}
}

func (s *Service) Layout() Layout { return s.layout }

func (s *Service) Status(ctx context.Context) (StatusResponse, error) {
	_ = s.layout.EnsureUpdaterScript()

	st, err := s.store.Load()
	if err != nil {
		return StatusResponse{}, err
	}
	s.resumeStaleUpdateIfNeeded(st)
	if st.Phase == PhaseChecking {
		st.Phase = PhaseIdle
		_ = s.store.Save(st)
	}
	resp := StatusResponse{
		CurrentVersion: version.Version,
		CurrentCommit:  version.Commit,
		Phase:          st.Phase,
		TargetVersion:  st.TargetVersion,
		Error:          st.Error,
		CanApply:       st.Phase == PhaseVerified,
		CanDownload:    st.Phase == PhaseIdle || st.Phase == PhaseFailed || st.Phase == PhaseRolledBack || st.Phase == PhaseDownloading,
	}
	if st.TotalBytes > 0 {
		resp.Progress = float64(st.DownloadedBytes) / float64(st.TotalBytes) * 100
	}
	if s.layout.HasPrevious() && s.layout.UpdaterReady() {
		if v := s.layout.PreviousPanelVersion(); v != "" {
			resp.RollbackVersion = v
			resp.PreviousVersion = v
			resp.CanRollback = !SameVersionLabel(v, version.Version)
		} else if st.PreviousVersion != "" {
			resp.PreviousVersion = st.PreviousVersion
			resp.RollbackVersion = st.PreviousVersion
			resp.CanRollback = !SameVersionLabel(st.PreviousVersion, version.Version)
		} else {
			resp.PreviousVersion = "previous"
			resp.RollbackVersion = "previous"
			resp.CanRollback = true
		}
	} else if st.PreviousVersion != "" {
		resp.PreviousVersion = st.PreviousVersion
	}
	if st.TargetVersion != "" && IsUpdateAvailable(st.TargetVersion, version.Version) {
		resp.Available = &ReleaseInfo{
			Version:     st.TargetVersion,
			DownloadURL: st.DownloadURL,
		}
	}
	return resp, nil
}

func (s *Service) Check(ctx context.Context) (StatusResponse, error) {
	if err := s.layout.EnsureDirs(); err != nil {
		return StatusResponse{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, CheckReleaseTimeout)
	defer cancel()

	st, err := s.store.Load()
	if err != nil {
		return StatusResponse{}, err
	}
	if st.Phase.IsActive() && st.Phase != PhaseDownloading && st.Phase != PhaseChecking {
		return s.Status(ctx)
	}

	rel, err := FetchLatestRelease(ctx, s.checkClient)
	if err != nil {
		return StatusResponse{}, err
	}

	st.TargetVersion = rel.Version
	st.DownloadURL = rel.DownloadURL
	st.PreviousVersion = version.Version
	st.Attempts = 0
	st.Phase = PhaseIdle
	if err := s.store.Save(st); err != nil {
		return StatusResponse{}, err
	}

	resp, err := s.Status(ctx)
	if err != nil {
		return StatusResponse{}, err
	}
	if IsUpdateAvailable(rel.Version, version.Version) {
		resp.Available = &rel
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
		checkCtx, cancel := context.WithTimeout(ctx, CheckReleaseTimeout)
		rel, err := FetchLatestRelease(checkCtx, s.checkClient)
		cancel()
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

	if !IsUpdateAvailable(st.TargetVersion, version.Version) {
		return StatusResponse{}, fmt.Errorf("already running version %s (release %s is not newer)", version.Version, st.TargetVersion)
	}

	st.Phase = PhaseDownloading
	st.ArchivePath = ArchivePath(s.layout.DownloadsDir, st.TargetVersion)
	st.Attempts++
	if err := s.store.Save(st); err != nil {
		return StatusResponse{}, err
	}

	var dlErr error
	for attempt := 0; attempt < maxDownloadAttempts; attempt++ {
		st, dlErr = DownloadArchive(ctx, s.downloadClient, st.DownloadURL, st.ArchivePath, st, func(p State) {
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
	if err := s.layout.EnsureUpdaterScript(); err != nil {
		return StatusResponse{}, fmt.Errorf("install updater script: %w", err)
	}
	if !s.layout.UpdaterReady() {
		return StatusResponse{}, fmt.Errorf("updater script missing: %s", s.layout.UpdaterScript)
	}

	st.Phase = PhaseApplying
	if err := s.store.Save(st); err != nil {
		return StatusResponse{}, err
	}

	if err := s.spawnUpdaterScript("apply"); err != nil {
		_ = s.store.Fail(st, PhaseFailed, err)
		return s.Status(ctx)
	}
	return s.Status(ctx)
}

func (s *Service) Rollback(ctx context.Context) (StatusResponse, error) {
	if err := s.layout.EnsureUpdaterScript(); err != nil {
		return StatusResponse{}, fmt.Errorf("install updater script: %w", err)
	}
	if !s.layout.HasPrevious() {
		return StatusResponse{}, fmt.Errorf("no previous version to rollback (panel.previous missing)")
	}
	if !s.layout.UpdaterReady() {
		return StatusResponse{}, fmt.Errorf("updater script missing: %s", s.layout.UpdaterScript)
	}
	st, err := s.store.Load()
	if err != nil {
		return StatusResponse{}, err
	}
	st.Phase = PhaseRestarting
	if err := s.store.Save(st); err != nil {
		return StatusResponse{}, err
	}
	if err := s.spawnUpdaterScript("rollback"); err != nil {
		_ = s.store.Fail(st, PhaseFailed, err)
		return s.Status(ctx)
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
		if s.layout.UpdaterReady() {
			go func() {
				if err := s.spawnUpdaterScript("resume"); err != nil {
					log.Printf("update resume spawn: %v", err)
				}
			}()
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

func (s *Service) runPostUpdate(ctx context.Context) {
	if s.PostUpdateHook == nil {
		return
	}
	if err := s.PostUpdateHook(ctx); err != nil {
		log.Printf("post-update hook: %v", err)
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
	if s.layout.UpdaterReady() {
		_ = s.spawnUpdaterScript("rollback")
	}
	return fmt.Errorf("health check failed: %s", result.Message)
}

func (s *Service) StartHealthCheckIfNeeded(ctx context.Context) {
	st, err := s.store.Load()
	if err != nil {
		return
	}
	switch st.Phase {
	case PhaseRestarting:
		st.Phase = PhaseHealthCheck
		_ = s.store.Save(st)
		go s.runPostUpdateAndHealthCheck()
	case PhaseHealthCheck:
		go s.runPostUpdateAndHealthCheck()
	}
}

func (s *Service) runPostUpdateAndHealthCheck() {
	time.Sleep(2 * time.Second)
	s.runPostUpdate(context.Background())
	_ = s.runHealthCheck(context.Background())
}
