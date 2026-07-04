package config

import "testing"

func TestMigrateConfigDefaultsFailOpen(t *testing.T) {
	cfg := DefaultPanelConfig()
	cfg.FailOpen = FailOpen{}

	MigrateConfigDefaults(&cfg)

	if cfg.FailOpen.Enabled == nil || !*cfg.FailOpen.Enabled {
		t.Fatal("expected fail_open enabled default")
	}
	if cfg.FailOpen.RestoreOnRecovery == nil || !*cfg.FailOpen.RestoreOnRecovery {
		t.Fatal("expected restore_on_recovery default")
	}
}

func TestMigrateConfigDefaultsPreservesExplicitFalse(t *testing.T) {
	falseVal := false
	cfg := DefaultPanelConfig()
	cfg.FailOpen = FailOpen{Enabled: &falseVal, RestoreOnRecovery: &falseVal}

	MigrateConfigDefaults(&cfg)

	if cfg.FailOpen.Enabled == nil || *cfg.FailOpen.Enabled {
		t.Fatal("expected explicit disabled preserved")
	}
}
