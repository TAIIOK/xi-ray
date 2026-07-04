package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

func TestFailOpenServiceMarker(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "failopen-marker")
	store, err := config.NewStore(filepath.Join(dir, "panel.json"))
	if err != nil {
		t.Fatal(err)
	}
	_ = store.Update(func(c *config.PanelConfig) error {
		c.FailOpen.MarkerPath = marker
		return nil
	})

	svc := NewFailOpenService(store)
	if svc.IsActive() {
		t.Fatal("expected inactive before enable")
	}

	if err := svc.Enable(context.Background()); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if !svc.IsActive() {
		t.Fatal("expected active after enable")
	}

	if err := svc.Disable(); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if svc.IsActive() {
		t.Fatal("expected inactive after disable")
	}
}

func TestFailOpenEnabledDefault(t *testing.T) {
	store, err := config.NewStore(filepath.Join(t.TempDir(), "panel.json"))
	if err != nil {
		t.Fatal(err)
	}
	svc := NewFailOpenService(store)
	if !svc.Enabled() {
		t.Fatal("expected fail-open enabled by default")
	}
}

func TestFailOpenDisabledExplicit(t *testing.T) {
	dir := t.TempDir()
	store, err := config.NewStore(filepath.Join(dir, "panel.json"))
	if err != nil {
		t.Fatal(err)
	}
	falseVal := false
	_ = store.Update(func(c *config.PanelConfig) error {
		c.FailOpen.Enabled = &falseVal
		return nil
	})
	svc := NewFailOpenService(store)
	if svc.Enabled() {
		t.Fatal("expected fail-open disabled")
	}
	if err := svc.MaybeEnable(context.Background()); err != nil {
		t.Fatalf("maybe enable: %v", err)
	}
	if _, err := os.Stat(store.Get().FailOpen.MarkerPathOrDefault()); !os.IsNotExist(err) {
		t.Fatal("expected no marker when disabled")
	}
}
