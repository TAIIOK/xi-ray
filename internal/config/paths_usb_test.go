package config

import "testing"

func TestPathsForUSB(t *testing.T) {
	p := PathsForUSB("/mnt/usb-test")
	if p.PanelDataDir != "/mnt/usb-test/xiaomi-vless" {
		t.Fatalf("home: %s", p.PanelDataDir)
	}
	if p.StartupScript != "/mnt/usb-test/xiaomi-vless/startup_xray_guest.sh" {
		t.Fatalf("startup: %s", p.StartupScript)
	}
}

func TestMigrateLegacyDataPaths(t *testing.T) {
	cfg := DefaultPanelConfig()
	cfg.Paths.StartupScript = "/data/startup_xray_guest.sh"
	cfg.Paths.PanelDataDir = "/data/xiaomi-vless"
	if !MigrateLegacyDataPaths(&cfg) {
		t.Fatal("expected migration")
	}
	if cfg.Paths.PanelDataDir != PanelHomeOnUSB(DefaultUSBMount) {
		t.Fatalf("got %s", cfg.Paths.PanelDataDir)
	}
}
