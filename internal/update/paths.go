package update

import (
	"os"
	"path/filepath"
)

const (
	repoDefault = "TAIIOK/xi-ray"
)

type Phase string

const (
	PhaseIdle        Phase = "idle"
	PhaseChecking    Phase = "checking"
	PhaseDownloading Phase = "downloading"
	PhaseExtracting  Phase = "extracting"
	PhaseVerified    Phase = "verified"
	PhaseApplying    Phase = "applying"
	PhaseRestarting  Phase = "restarting"
	PhaseHealthCheck Phase = "health_check"
	PhaseCompleted   Phase = "completed"
	PhaseFailed      Phase = "failed"
	PhaseRolledBack  Phase = "rolled_back"
)

type State struct {
	Phase           Phase  `json:"phase"`
	TargetVersion   string `json:"target_version,omitempty"`
	PreviousVersion string `json:"previous_version,omitempty"`
	DownloadURL     string `json:"download_url,omitempty"`
	ArchivePath     string `json:"archive_path,omitempty"`
	DownloadedBytes int64  `json:"downloaded_bytes,omitempty"`
	TotalBytes      int64  `json:"total_bytes,omitempty"`
	Checksum        string `json:"checksum,omitempty"`
	StartedAt       string `json:"started_at,omitempty"`
	UpdatedAt       string `json:"updated_at,omitempty"`
	Error           string `json:"error,omitempty"`
	Attempts        int    `json:"attempts,omitempty"`
}

func (p Phase) IsActive() bool {
	switch p {
	case PhaseIdle, PhaseCompleted, PhaseRolledBack:
		return false
	default:
		return true
	}
}

func (p Phase) NeedsResume() bool {
	switch p {
	case PhaseDownloading, PhaseExtracting, PhaseApplying, PhaseRestarting, PhaseHealthCheck:
		return true
	default:
		return false
	}
}

type Layout struct {
	Home           string
	PanelBin       string
	PanelPrevious  string
	ScriptsDir     string
	ScriptsPrevDir string
	UpdatesDir     string
	StatePath      string
	StagingDir     string
	DownloadsDir   string
	UpdaterScript  string
	ConfigPath     string
}

func LayoutForHome(home, configPath string) Layout {
	home = filepath.Clean(home)
	return Layout{
		Home:           home,
		PanelBin:       filepath.Join(home, "panel"),
		PanelPrevious:  filepath.Join(home, "panel.previous"),
		ScriptsDir:     filepath.Join(home, "scripts"),
		ScriptsPrevDir: filepath.Join(home, "scripts.previous"),
		UpdatesDir:     filepath.Join(home, "updates"),
		StatePath:      filepath.Join(home, "updates", "state.json"),
		StagingDir:     filepath.Join(home, "updates", "staging"),
		DownloadsDir:   filepath.Join(home, "updates", "downloads"),
		UpdaterScript:  filepath.Join(home, "panel-updater.sh"),
		ConfigPath:     configPath,
	}
}

func (l Layout) EnsureDirs() error {
	for _, dir := range []string{l.UpdatesDir, l.StagingDir, l.DownloadsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func (l Layout) HasPrevious() bool {
	_, err := os.Stat(l.PanelPrevious)
	return err == nil
}

func ArchivePath(downloadsDir, version string) string {
	return filepath.Join(downloadsDir, "v"+trimVersion(version)+".tar.gz")
}

func trimVersion(v string) string {
	for len(v) > 0 && v[0] == 'v' {
		v = v[1:]
	}
	return v
}
