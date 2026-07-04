package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/taiiok/xiaomi-vless/internal/config"
	"github.com/taiiok/xiaomi-vless/internal/xray"
)

type FailOpenService struct {
	store *config.Store
}

func NewFailOpenService(store *config.Store) *FailOpenService {
	return &FailOpenService{store: store}
}

func (s *FailOpenService) Enabled() bool {
	return s.store.Get().FailOpen.EnabledOrDefault()
}

func (s *FailOpenService) markerPath() string {
	return s.store.Get().FailOpen.MarkerPathOrDefault()
}

func (s *FailOpenService) teardownScriptPath() string {
	cfg := s.store.Get()
	if cfg.Paths.IptablesScript != "" {
		return xray.IPTablesTeardownScriptPath(cfg.Paths.IptablesScript)
	}
	return ""
}

func (s *FailOpenService) IsActive() bool {
	_, err := os.Stat(s.markerPath())
	return err == nil
}

func (s *FailOpenService) Enable(ctx context.Context) error {
	if err := s.runTeardown(ctx); err != nil {
		return err
	}
	marker := s.markerPath()
	if err := config.EnsureDir(filepath.Dir(marker), config.PanelDirPerm); err != nil {
		return fmt.Errorf("fail-open marker dir: %w", err)
	}
	f, err := os.OpenFile(marker, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("fail-open marker: %w", err)
	}
	_, _ = fmt.Fprintf(f, "enabled_at=%s\n", time.Now().Format(time.RFC3339))
	if err := f.Close(); err != nil {
		return fmt.Errorf("fail-open marker close: %w", err)
	}
	return nil
}

func (s *FailOpenService) Disable() error {
	if err := os.Remove(s.markerPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *FailOpenService) runTeardown(ctx context.Context) error {
	script := s.teardownScriptPath()
	if script != "" {
		if st, err := os.Stat(script); err == nil && !st.IsDir() {
			out, err := exec.CommandContext(ctx, "sh", script).CombinedOutput()
			if err != nil {
				return fmt.Errorf("iptables teardown failed: %w\n%s", err, strings.TrimSpace(string(out)))
			}
			return nil
		}
	}
	return s.runInlineTeardown(ctx)
}

func (s *FailOpenService) runInlineTeardown(ctx context.Context) error {
	script := xray.GenerateIPTablesTeardownScript()
	out, err := exec.CommandContext(ctx, "sh", "-c", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("inline iptables teardown failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s *FailOpenService) MaybeEnable(ctx context.Context) error {
	if !s.Enabled() {
		return nil
	}
	return s.Enable(ctx)
}
