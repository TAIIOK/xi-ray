package config

// MigrateConfigDefaults fills new config sections after panel updates without overwriting user choices.
func MigrateConfigDefaults(cfg *PanelConfig) {
	if cfg.FailOpen.Enabled == nil {
		cfg.FailOpen.Enabled = boolPtr(true)
	}
	if cfg.FailOpen.RestoreOnRecovery == nil {
		cfg.FailOpen.RestoreOnRecovery = boolPtr(true)
	}
	cfg.Normalize()
}
