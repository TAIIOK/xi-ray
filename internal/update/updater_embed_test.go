package update

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureUpdaterScript(t *testing.T) {
	dir := t.TempDir()
	layout := LayoutForHome(dir, filepath.Join(dir, "panel.json"))
	if layout.UpdaterReady() {
		t.Fatal("expected missing updater")
	}
	if err := layout.EnsureUpdaterScript(); err != nil {
		t.Fatal(err)
	}
	if !layout.UpdaterReady() {
		t.Fatal("expected updater installed")
	}
	if err := os.WriteFile(layout.UpdaterScript, []byte("# stale\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := layout.EnsureUpdaterScript(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(layout.UpdaterScript)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("systemctl restart xiaomi-vless-panel.service")) {
		t.Fatal("expected embedded updater to replace stale script")
	}
}

func TestEnsureUpdaterScriptKeepsExistingWhenTestEnv(t *testing.T) {
	t.Setenv("XIAOMI_VLESS_KEEP_UPDATER_SCRIPT", "1")
	dir := t.TempDir()
	layout := LayoutForHome(dir, filepath.Join(dir, "panel.json"))
	stub := []byte("#!/bin/sh\nexit 0\n")
	if err := os.WriteFile(layout.UpdaterScript, stub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := layout.EnsureUpdaterScript(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(layout.UpdaterScript)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, stub) {
		t.Fatal("expected test stub updater to be preserved")
	}
}
