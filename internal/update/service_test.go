package update

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/taiiok/xiaomi-vless/internal/config"
	"github.com/taiiok/xiaomi-vless/internal/version"
)

func TestStatusRecoversCheckingAndExposesAvailable(t *testing.T) {
	dir := t.TempDir()
	layout := LayoutForHome(dir, filepath.Join(dir, "panel.json"))
	store := NewStateStore(layout.StatePath)
	if err := store.Save(State{
		Phase:         PhaseChecking,
		TargetVersion: "0.1.1",
		DownloadURL:   "https://example.com/bundle.tar.gz",
	}); err != nil {
		t.Fatal(err)
	}

	svc := NewService(dir, layout.ConfigPath, func() config.PanelConfig { return config.PanelConfig{} })
	resp, err := svc.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if resp.Phase != PhaseIdle {
		t.Fatalf("phase = %q, want idle", resp.Phase)
	}
	if resp.Available == nil || resp.Available.Version != "0.1.1" {
		t.Fatalf("available = %+v", resp.Available)
	}
	if !resp.CanDownload {
		t.Fatal("expected can_download when update is available")
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Phase != PhaseIdle {
		t.Fatalf("persisted phase = %q, want idle", loaded.Phase)
	}
}

func TestStatusHidesOlderRelease(t *testing.T) {
	old := version.Version
	version.Version = "v0.3.0-dirty"
	t.Cleanup(func() { version.Version = old })

	dir := t.TempDir()
	layout := LayoutForHome(dir, filepath.Join(dir, "panel.json"))
	store := NewStateStore(layout.StatePath)
	if err := store.Save(State{
		Phase:         PhaseIdle,
		TargetVersion: "0.2.0",
		DownloadURL:   "https://example.com/bundle.tar.gz",
	}); err != nil {
		t.Fatal(err)
	}

	svc := NewService(dir, layout.ConfigPath, func() config.PanelConfig { return config.PanelConfig{} })
	resp, err := svc.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if resp.Available != nil {
		t.Fatalf("available = %+v, want nil when release is older", resp.Available)
	}
}
