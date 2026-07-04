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
		rest = strings.TrimSpace(rest)
		if rest != "" && rest[0] != 'v' && rest[0] != 'V' {
			return "v" + rest
		}
		return rest
	}
	return line
}

// SameVersionLabel reports whether two version strings refer to the same release label.
func SameVersionLabel(a, b string) bool {
	return normalizeVersionLabel(a) == normalizeVersionLabel(b)
}

func normalizeVersionLabel(v string) string {
	v = strings.TrimSpace(v)
	for len(v) > 0 && (v[0] == 'v' || v[0] == 'V') {
		v = v[1:]
	}
	return v
}
