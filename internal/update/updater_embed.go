package update

import (
	"bytes"
	_ "embed"
	"os"
)

//go:embed scripts/panel-updater.sh
var embeddedUpdaterScript []byte

func (l Layout) EnsureUpdaterScript() error {
	if len(embeddedUpdaterScript) == 0 {
		return nil
	}
	if data, err := os.ReadFile(l.UpdaterScript); err == nil && bytes.Equal(data, embeddedUpdaterScript) {
		return nil
	}
	if err := os.MkdirAll(l.Home, 0o755); err != nil {
		return err
	}
	return os.WriteFile(l.UpdaterScript, embeddedUpdaterScript, 0o755)
}

func (l Layout) UpdaterReady() bool {
	info, err := os.Stat(l.UpdaterScript)
	return err == nil && info.Size() > 0
}
