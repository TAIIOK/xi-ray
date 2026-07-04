package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/taiiok/xiaomi-vless/internal/config"
	"github.com/taiiok/xiaomi-vless/internal/xray"
)

func TestRunPostUpdateWritesTeardownScript(t *testing.T) {
	dir := t.TempDir()
	store, err := config.NewStore(filepath.Join(dir, "panel.json"))
	if err != nil {
		t.Fatal(err)
	}
	iptables := filepath.Join(dir, "xray-guest-iptables.sh")
	_ = store.Update(func(c *config.PanelConfig) error {
		c.Paths.IptablesScript = iptables
		c.Paths.PanelDataDir = dir
		c.Setup.OnboardingCompleted = false
		return nil
	})

	panel := NewPanelService(store)
	if err := panel.RunPostUpdate(context.Background()); err != nil {
		t.Fatal(err)
	}

	down := xray.IPTablesTeardownScriptPath(iptables)
	if _, err := os.Stat(down); err != nil {
		t.Fatalf("teardown script missing: %v", err)
	}
}
