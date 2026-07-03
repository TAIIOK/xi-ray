package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/taiiok/xiaomi-vless/internal/config"
	"github.com/taiiok/xiaomi-vless/internal/setup"
	"github.com/taiiok/xiaomi-vless/internal/xray"
)

type ApplyService struct {
	store *config.Store
}

func NewApplyService(store *config.Store) *ApplyService {
	return &ApplyService{store: store}
}

type ApplyResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

func (s *ApplyService) Apply(ctx context.Context) (*ApplyResult, error) {
	if err := s.store.Update(func(cfg *config.PanelConfig) error {
		sanitizeSelection(cfg)
		return nil
	}); err != nil {
		return nil, err
	}

	cfg := s.store.Get()
	if err := config.ValidatePaths(cfg.Paths); err != nil {
		return nil, err
	}

	nodes, err := xray.ActiveNodes(cfg)
	if err != nil {
		return &ApplyResult{OK: false, Message: err.Error()}, nil
	}

	xrayData, err := xray.Generate(cfg)
	if err != nil {
		return &ApplyResult{OK: false, Message: err.Error()}, nil
	}

	iptablesScript, err := xray.GenerateIPTablesScript(cfg, nodes)
	if err != nil {
		return &ApplyResult{OK: false, Message: err.Error()}, nil
	}

	if err := config.EnsureDir(cfg.Paths.PanelDataDir, config.PanelDirPerm); err != nil {
		return &ApplyResult{OK: false, Message: fmt.Sprintf("panel data dir: %v", err)}, nil
	}

	tmpPath := xray.StagingConfigPath(cfg.Paths.PanelDataDir, cfg.Paths.XrayConfig)
	lastPath := xray.LastGeneratedConfigPath(cfg.Paths.PanelDataDir)
	if err := writeConfigFile(tmpPath, xrayData); err != nil {
		return &ApplyResult{OK: false, Message: fmt.Sprintf("write staging config: %v", err)}, nil
	}
	_ = writeConfigFile(lastPath, xrayData) // debug copy; best-effort

	if err := setup.EnsureGeoAssets(ctx, cfg.Paths.XrayBin); err != nil {
		return &ApplyResult{OK: false, Message: err.Error()}, nil
	}

	binDir := setup.XrayBinDir(cfg.Paths.XrayBin)
	testCmd := exec.CommandContext(ctx, cfg.Paths.XrayBin, "run", "-test", "-c", tmpPath)
	testCmd.Dir = binDir
	testCmd.Env = append(os.Environ(), "XRAY_LOCATION_ASSET="+binDir)
	if out, err := testCmd.CombinedOutput(); err != nil {
		return &ApplyResult{
			OK:      false,
			Message: fmt.Sprintf("xray test failed: %s (staging left at %s)", strings.TrimSpace(string(out)), tmpPath),
		}, nil
	}

	if err := os.WriteFile(cfg.Paths.IptablesScript, []byte(iptablesScript), 0o755); err != nil {
		return nil, err
	}

	_ = os.Remove(cfg.Paths.XrayConfig + ".new") // legacy temp name from older panel builds
	if err := writeConfigFile(cfg.Paths.XrayConfig, xrayData); err != nil {
		return &ApplyResult{OK: false, Message: fmt.Sprintf("write xray config to USB: %v", err)}, nil
	}
	// Staging in /data is kept for inspection; safe to remove manually.

	setup.EnsureSystemScripts(cfg.Paths)
	ensureXrayLogFiles(cfg)

	if err := s.restartStack(context.WithoutCancel(ctx), cfg); err != nil {
		return &ApplyResult{OK: false, Message: err.Error()}, nil
	}

	if err := s.waitHealthy(ctx, 15*time.Second); err != nil {
		detail := err.Error()
		if IsXrayRunning() {
			detail += "\n(xray process is running; check routing/outbound or SOCKS port)"
		} else {
			detail += "\n" + xrayFailureDetails(s.store.Get())
		}
		return &ApplyResult{OK: false, Message: fmt.Sprintf("applied but health check failed: %s", detail)}, nil
	}

	return &ApplyResult{OK: true, Message: "config and iptables applied, xray restarted"}, nil
}

func writeConfigFile(path string, data []byte) error {
	if err := config.EnsureDir(filepath.Dir(path), config.PanelDirPerm); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, config.ConfigFilePerm)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Chmod(path, config.ConfigFilePerm)
}

func (s *ApplyService) Restart(ctx context.Context) error {
	cfg := s.store.Get()
	setup.EnsureSystemScripts(cfg.Paths)
	ensureXrayLogFiles(cfg)
	return s.restartStack(context.WithoutCancel(ctx), cfg)
}

func (s *ApplyService) restartStack(ctx context.Context, cfg config.PanelConfig) error {
	if cfg.Paths.XrayBin == "" || cfg.Paths.XrayConfig == "" {
		return fmt.Errorf("restart: xray paths not configured")
	}

	sysctl := setup.SysctlPath(cfg.Paths)
	if st, err := os.Stat(sysctl); err == nil && !st.IsDir() {
		if out, err := exec.CommandContext(ctx, "sh", sysctl).CombinedOutput(); err != nil {
			return fmt.Errorf("sysctl setup failed: %w\n%s", err, strings.TrimSpace(string(out)))
		}
	}

	binDir := setup.XrayBinDir(cfg.Paths.XrayBin)
	stopXray(ctx)

	cmd := exec.CommandContext(ctx, cfg.Paths.XrayBin, "run", "-c", cfg.Paths.XrayConfig)
	cmd.Dir = binDir
	cmd.Env = append(os.Environ(), "XRAY_LOCATION_ASSET="+binDir)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start xray: %w", err)
	}

	if !waitForXray(5 * time.Second) {
		return fmt.Errorf("xray exited immediately after start\n%s", xrayFailureDetails(cfg))
	}

	if cfg.Paths.IptablesScript != "" {
		if out, err := exec.CommandContext(ctx, "sh", cfg.Paths.IptablesScript).CombinedOutput(); err != nil {
			return fmt.Errorf("iptables apply failed: %w\n%s", err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

func stopXray(ctx context.Context) {
	_ = exec.CommandContext(ctx, "killall", "xray").Run()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !IsXrayRunning() {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	_ = exec.CommandContext(ctx, "killall", "-9", "xray").Run()
	time.Sleep(500 * time.Millisecond)
}

func waitForXray(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if IsXrayRunning() {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return IsXrayRunning()
}

func ensureXrayLogFiles(cfg config.PanelConfig) {
	for _, path := range []string{cfg.Logs.XrayAccess, cfg.Logs.XrayError, cfg.Logs.Startup} {
		if path == "" {
			continue
		}
		_ = config.EnsureDir(filepath.Dir(path), config.PanelDirPerm)
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, config.ConfigFilePerm)
		if err == nil {
			_ = f.Close()
		}
	}
}

func xrayFailureDetails(cfg config.PanelConfig) string {
	var parts []string
	for _, item := range []struct {
		label, path string
	}{
		{"xray error log", cfg.Logs.XrayError},
		{"startup log", cfg.Logs.Startup},
		{"xray access log", cfg.Logs.XrayAccess},
	} {
		if item.path == "" {
			continue
		}
		if tail := tailFileText(item.path, 20); tail != "" {
			parts = append(parts, fmt.Sprintf("--- %s (%s) ---\n%s", item.label, item.path, tail))
		}
	}
	if len(parts) == 0 {
		return "(no log output captured — check paths in panel settings)"
	}
	return strings.Join(parts, "\n")
}

func tailFileText(path string, maxLines int) string {
	lines, _, err := tailFile(path, maxLines)
	if err != nil || len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (s *ApplyService) socksAddr() string {
	cfg := s.store.Get()
	return fmt.Sprintf("socks5h://127.0.0.1:%d", cfg.Iptables.SOCKSPort)
}

func (s *ApplyService) waitHealthy(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	socks := s.socksAddr()
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if IsXrayRunning() {
			_, err := SOCKSProbeAt(ctx, socks, "https://www.google.com/generate_204")
			if err == nil {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for healthy xray")
}

func IsXrayRunning() bool {
	out, err := exec.Command("pidof", "xray").Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

func SOCKSProbe(ctx context.Context, targetURL string) (string, error) {
	return SOCKSProbeAt(ctx, "socks5h://127.0.0.1:10808", targetURL)
}

func SOCKSProbeAt(ctx context.Context, proxy, targetURL string) (string, error) {
	cmd := exec.CommandContext(ctx, "curl", "-4", "-sS", "-o", "/dev/null", "-w", "%{http_code}",
		"--connect-timeout", "5", "-x", proxy, targetURL)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	code := strings.TrimSpace(string(out))
	if code == "000" || code == "" {
		return "", fmt.Errorf("probe failed")
	}
	if code >= "400" && code != "204" {
		return "", fmt.Errorf("HTTP %s", code)
	}
	return code, nil
}
