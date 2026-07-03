package setup

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

// EnsureSystemScripts installs embedded startup/iptables/sysctl scripts when missing.
func EnsureSystemScripts(p config.Paths) []SetupAction {
	var actions []SetupAction
	sysctl := SysctlPath(p)
	for _, item := range []struct {
		dest, script string
	}{
		{p.StartupScript, "startup_xray_guest.sh"},
		{p.IptablesScript, "xray-guest-iptables.sh"},
		{sysctl, "xray-guest-sysctl.sh"},
	} {
		if item.dest == "" {
			continue
		}
		existed := fileExists(item.dest)
		err := installScript(item.dest, item.script)
		msg := "ready"
		ok := err == nil
		if err != nil {
			msg = err.Error()
		} else if !existed {
			msg = "installed"
		}
		actions = append(actions, SetupAction{Action: "ensure", Path: item.dest, OK: ok, Message: msg})
	}
	_ = writeXrayEnv(p)
	return actions
}

func SysctlPath(p config.Paths) string {
	if p.IptablesScript != "" {
		return filepath.Join(filepath.Dir(p.IptablesScript), "xray-guest-sysctl.sh")
	}
	if p.PanelDataDir != "" {
		return filepath.Join(p.PanelDataDir, "xray-guest-sysctl.sh")
	}
	return filepath.Join(config.PanelHomeOnUSB(config.DefaultUSBMount), "xray-guest-sysctl.sh")
}

// AdaptPathsForEnvironment uses a writable runtime dir for local dev without USB.
func AdaptPathsForEnvironment(cfg *config.PanelConfig, configPath string) bool {
	home := cfg.Paths.PanelDataDir
	if home != "" && pathWritable(home) && pathWritable(filepath.Dir(cfg.Paths.StartupScript)) {
		return false
	}
	base := localRuntimeDir(configPath)
	cfg.Paths.StartupScript = filepath.Join(base, "startup_xray_guest.sh")
	cfg.Paths.IptablesScript = filepath.Join(base, "xray-guest-iptables.sh")
	if cfg.Paths.PanelDataDir == "" || strings.HasPrefix(cfg.Paths.PanelDataDir, "/data/") || !pathWritable(cfg.Paths.PanelDataDir) {
		cfg.Paths.PanelDataDir = filepath.Join(base, "xiaomi-vless")
	}
	if cfg.Logs.Startup == "" || strings.HasPrefix(cfg.Logs.Startup, "/data/") {
		cfg.Logs.Startup = filepath.Join(cfg.Paths.PanelDataDir, "xray-startup.log")
	}
	if cfg.Logs.Panel == "" || strings.HasPrefix(cfg.Logs.Panel, "/data/") {
		cfg.Logs.Panel = filepath.Join(cfg.Paths.PanelDataDir, "panel.log")
	}
	if cfg.Logs.XrayAccess == "" || strings.HasPrefix(cfg.Logs.XrayAccess, "/data/") {
		cfg.Logs.XrayAccess = filepath.Join(cfg.Paths.PanelDataDir, "xray-access.log")
	}
	if cfg.Logs.XrayError == "" || strings.HasPrefix(cfg.Logs.XrayError, "/data/") {
		cfg.Logs.XrayError = filepath.Join(cfg.Paths.PanelDataDir, "xray-error.log")
	}
	cfg.Normalize()
	return true
}

func localRuntimeDir(configPath string) string {
	dir := filepath.Dir(configPath)
	if dir == "" || dir == "." {
		dir, _ = os.Getwd()
	}
	return filepath.Join(dir, ".xiaomi-vless-runtime")
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func hintForMissing(key string) string {
	switch key {
	case "xray_bin", "xray_config", "geoip_dat", "geosite_dat":
		return "выберите USB и нажмите «Скачать Xray», или укажите путь вручную"
	case "startup_script", "iptables_script":
		return "нажмите «Установить недостающее» — скрипты кладутся в xiaomi-vless/ на USB"
	default:
		return "нажмите «Установить недостающее»"
	}
}

// BootstrapOnStart ensures system scripts exist; on local dev relocates paths off /data.
func BootstrapOnStart(store interface {
	Get() config.PanelConfig
	Update(func(*config.PanelConfig) error) error
}, configPath string, localDev bool) {
	cfg := store.Get()
	if config.UsesLegacyDataPaths(cfg) {
		adapted := cfg
		if config.MigrateLegacyDataPaths(&adapted) {
			_ = store.Update(func(c *config.PanelConfig) error {
				c.Paths = adapted.Paths
				c.Logs = adapted.Logs
				return nil
			})
			cfg = store.Get()
			log.Printf("migrated panel paths to USB: %s", cfg.Paths.PanelDataDir)
		}
	}

	if localDev || !pathWritable(cfg.Paths.PanelDataDir) {
		adapted := cfg
		if AdaptPathsForEnvironment(&adapted, configPath) {
			_ = store.Update(func(c *config.PanelConfig) error {
				c.Paths = adapted.Paths
				c.Logs = adapted.Logs
				return nil
			})
			cfg = store.Get()
			log.Printf("using writable script paths under %s", filepath.Dir(cfg.Paths.StartupScript))
		}
	}

	for _, a := range EnsureSystemScripts(cfg.Paths) {
		if a.OK {
			if a.Message == "installed" {
				log.Printf("installed script: %s", a.Path)
			}
			continue
		}
		log.Printf("script setup: %s — %s", a.Path, a.Message)
	}

	if cfg.Paths.XrayBin != "" && fileExists(cfg.Paths.XrayBin) {
		if err := migrateGeoAssetsToBin(cfg.Paths.XrayBin); err != nil {
			log.Printf("geo assets migration: %v", err)
		}
	}
}
