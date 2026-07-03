package service

import (
	"testing"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

func TestResetPanelConfigOnboarding(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/panel.json"
	store, err := config.NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetPassword("admin", "secret"); err != nil {
		t.Fatal(err)
	}
	if err := store.Update(func(cfg *config.PanelConfig) error {
		cfg.Setup.OnboardingCompleted = true
		cfg.Nodes = []config.Node{{ID: "stale", Protocol: "vless"}}
		cfg.Selection.ActiveNodeIDs = []string{"stale"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := ResetPanelConfig(store, "onboarding"); err != nil {
		t.Fatal(err)
	}
	cfg := store.Get()
	if cfg.Setup.OnboardingCompleted {
		t.Fatal("expected onboarding incomplete")
	}
	if len(cfg.Nodes) != 0 {
		t.Fatalf("expected no nodes, got %d", len(cfg.Nodes))
	}
	if !store.IsDefaultPassword() {
		t.Fatal("expected default password")
	}
}
