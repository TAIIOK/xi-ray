package update

import (
	"os"
	"os/exec"
	"strings"
)

// PreviousPanelVersion returns semver from panel.previous (-version output), or empty.
func (l Layout) PreviousPanelVersion() string {
	if _, err := os.Stat(l.PanelPrevious); err != nil {
		return ""
	}
	out, err := exec.Command(l.PanelPrevious, "-version").Output()
	if err != nil {
		return ""
	}
	return parsePanelVersionLine(string(out))
}

func parsePanelVersionLine(s string) string {
	line := strings.TrimSpace(strings.Split(s, "\n")[0])
	if line == "" {
		return ""
	}
	// xiaomi-vless v0.3.0 (abc123, 2026-07-04T...)
	const prefix = "xiaomi-vless "
	if strings.HasPrefix(line, prefix) {
		rest := strings.TrimPrefix(line, prefix)
		if idx := strings.IndexByte(rest, ' '); idx > 0 {
			rest = rest[:idx]
		}
		return strings.TrimPrefix(strings.TrimSpace(rest), "v")
	}
	return line
}
