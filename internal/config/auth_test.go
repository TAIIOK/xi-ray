package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEmptyPasswordHashDefaultsToAdmin(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "panel.json")
	example := `{
  "version": 2,
  "auth": {"username": "admin", "password_hash": ""},
  "setup": {"onboarding_completed": false},
  "paths": {"panel_data_dir": "` + dir + `"},
  "network": {"listen_addr": "127.0.0.1:7777"},
  "iptables": {},
  "selection": {"mode": "single", "active_node_ids": [], "fallback_order": []},
  "routing": {}
}`
	if err := os.WriteFile(path, []byte(example), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if !store.CheckPassword("admin", "admin") {
		t.Fatal("expected admin/admin to work after load")
	}
	if store.Get().Auth.PasswordHash == "" {
		t.Fatal("expected password hash to be persisted")
	}
	if store.Get().Setup.OnboardingCompleted {
		t.Fatal("onboarding should stay open for default password")
	}
}
