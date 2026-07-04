package update

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"time"
)

const staleUpdateResumeAfter = 90 * time.Second

// spawnUpdaterScript runs panel-updater.sh detached; output goes to panel-update.log.
func (s *Service) spawnUpdaterScript(action string) error {
	script := s.layout.UpdaterScript
	logPath := filepath.Join(s.layout.Home, "panel-update.log")
	shellCmd := fmt.Sprintf("%q %q >> %q 2>&1", script, action, logPath)
	cmd := exec.Command("nohup", "sh", "-c", shellCmd)
	cmd.Dir = s.layout.Home
	return cmd.Start()
}

func (s *Service) resumeStaleUpdateIfNeeded(st State) {
	if !st.Phase.NeedsResume() {
		return
	}
	updated, err := time.Parse(time.RFC3339, st.UpdatedAt)
	if err != nil || time.Since(updated) < staleUpdateResumeAfter {
		return
	}
	s.staleResumeMu.Lock()
	defer s.staleResumeMu.Unlock()
	if time.Since(s.lastStaleResume) < time.Minute {
		return
	}
	s.lastStaleResume = time.Now()
	go func() {
		if err := s.spawnUpdaterScript("resume"); err != nil {
			// best-effort recovery; next Status poll will retry
			_ = err
		}
	}()
}
