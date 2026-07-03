package update

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

type HealthResult struct {
	OK      bool     `json:"ok"`
	Message string   `json:"message"`
	Checks  []string `json:"checks,omitempty"`
}

func RunHealthChecks(ctx context.Context, cfg config.PanelConfig, layout Layout) HealthResult {
	checks := make([]string, 0, 4)

	if _, err := os.Stat(layout.ConfigPath); err != nil {
		return HealthResult{OK: false, Message: "panel.json missing", Checks: checks}
	}
	checks = append(checks, "panel.json ok")

	if cfg.Paths.XrayConfig != "" {
		if _, err := os.Stat(cfg.Paths.XrayConfig); err == nil {
			if err := xrayTest(cfg.Paths.XrayBin, cfg.Paths.XrayConfig); err != nil {
				return HealthResult{OK: false, Message: "xray -test failed: " + err.Error(), Checks: checks}
			}
			checks = append(checks, "xray config ok")
		}
	}

	if cfg.Paths.XrayBin != "" {
		if _, err := os.Stat(cfg.Paths.XrayBin); err != nil {
			return HealthResult{OK: false, Message: "xray binary missing", Checks: checks}
		}
		checks = append(checks, "xray binary ok")
	}

	if _, err := os.Stat(layout.PanelBin); err != nil {
		return HealthResult{OK: false, Message: "panel binary missing", Checks: checks}
	}
	checks = append(checks, "panel binary ok")

	return HealthResult{OK: true, Message: "all checks passed", Checks: checks}
}

func xrayTest(bin, configPath string) error {
	if strings.TrimSpace(bin) == "" || strings.TrimSpace(configPath) == "" {
		return nil
	}
	cmd := exec.CommandContext(context.Background(), bin, "-test", "-config", configPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
