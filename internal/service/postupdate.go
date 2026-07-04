package service

import (
	"context"
	"log"
	"os"

	"github.com/taiiok/xiaomi-vless/internal/config"
	"github.com/taiiok/xiaomi-vless/internal/setup"
	"github.com/taiiok/xiaomi-vless/internal/xray"
)

// RunPostUpdate applies migrations and stack changes after a panel self-update.
// Safe to call multiple times; skips VPN apply when onboarding is not finished.
func (p *PanelService) RunPostUpdate(ctx context.Context) error {
	if err := p.store.Update(func(cfg *config.PanelConfig) error {
		config.MigrateConfigDefaults(cfg)
		return nil
	}); err != nil {
		return err
	}

	cfg := p.store.Get()
	for _, action := range setup.EnsureSystemScripts(cfg.Paths) {
		if !action.OK {
			log.Printf("post-update: script %s: %s", action.Path, action.Message)
		}
	}

	if err := writeIPTablesTeardownScript(cfg); err != nil {
		log.Printf("post-update: teardown script: %v", err)
	}

	if !cfg.Setup.OnboardingCompleted {
		log.Printf("post-update: onboarding not completed — skipped VPN apply")
		return nil
	}

	if err := config.ValidatePaths(cfg.Paths); err != nil {
		log.Printf("post-update: paths invalid — skipped VPN apply: %v", err)
		return nil
	}

	result, err := p.apply.Apply(ctx)
	if err != nil {
		log.Printf("post-update: apply error: %v", err)
		return nil
	}
	if result != nil && !result.OK {
		log.Printf("post-update: apply: %s", result.Message)
		return nil
	}
	log.Printf("post-update: VPN stack applied")
	return nil
}

func writeIPTablesTeardownScript(cfg config.PanelConfig) error {
	if cfg.Paths.IptablesScript == "" {
		return nil
	}
	path := xray.IPTablesTeardownScriptPath(cfg.Paths.IptablesScript)
	return os.WriteFile(path, []byte(xray.GenerateIPTablesTeardownScript()), 0o755)
}
