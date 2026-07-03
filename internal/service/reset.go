package service

import (
	"fmt"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

// ResetPanelConfig resets parts of panel.json without reinstalling the binary.
// Modes:
//   - onboarding — admin/admin, onboarding again, clear nodes/subscriptions/selection; keep paths/network/routing
//   - full — factory defaults; keep paths and logs only
func ResetPanelConfig(store *config.Store, mode string) error {
	switch mode {
	case "onboarding":
		def := config.DefaultPanelConfig()
		if err := store.Update(func(cfg *config.PanelConfig) error {
			cfg.Auth = def.Auth
			cfg.Setup = def.Setup
			cfg.Subscriptions = nil
			cfg.Nodes = nil
			cfg.Selection = def.Selection
			return nil
		}); err != nil {
			return err
		}
		return store.SetPassword("admin", "admin")

	case "full":
		old := store.Get()
		def := config.DefaultPanelConfig()
		def.Paths = old.Paths
		if old.Logs.Startup != "" || old.Logs.Panel != "" {
			def.Logs = old.Logs
		}
		store.Replace(def)
		return store.Save()

	default:
		return fmt.Errorf("unknown reset mode %q (use: onboarding, full)", mode)
	}
}
