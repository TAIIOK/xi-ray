package config

import (
	"path/filepath"
	"strings"
)

const DefaultUSBMount = "/mnt/usb-ed49605f"

// USBMountFromXrayBin returns /mnt/usb-xxx from .../xray/bin/xray.
func USBMountFromXrayBin(xrayBin string) string {
	xrayBin = filepath.Clean(xrayBin)
	if strings.HasSuffix(xrayBin, "/xray/bin/xray") {
		return strings.TrimSuffix(xrayBin, "/xray/bin/xray")
	}
	return ""
}

// PanelHomeOnUSB is the panel/scripts/logs directory on the USB stick.
func PanelHomeOnUSB(usbMount string) string {
	return filepath.Join(strings.TrimRight(strings.TrimSpace(usbMount), "/"), "xiaomi-vless")
}

// PathsForUSB builds all runtime paths under the USB mount.
func PathsForUSB(usbMount string) Paths {
	usbMount = strings.TrimRight(strings.TrimSpace(usbMount), "/")
	home := PanelHomeOnUSB(usbMount)
	return Paths{
		XrayBin:        filepath.Join(usbMount, "xray/bin/xray"),
		XrayConfig:     filepath.Join(usbMount, "xray/config.json"),
		StartupScript:  filepath.Join(home, "startup_xray_guest.sh"),
		IptablesScript: filepath.Join(home, "xray-guest-iptables.sh"),
		PanelDataDir:   home,
	}
}

// LogsForPanelHome returns log paths inside panel home on USB.
func LogsForPanelHome(home string) Logs {
	home = strings.TrimRight(home, "/")
	return Logs{
		XrayAccess: filepath.Join(home, "xray-access.log"),
		XrayError:  filepath.Join(home, "xray-error.log"),
		Startup:    filepath.Join(home, "xray-startup.log"),
		Panel:      filepath.Join(home, "panel.log"),
	}
}

// PanelConfigPathOnUSB is the default panel.json location on USB.
func PanelConfigPathOnUSB(usbMount string) string {
	return filepath.Join(PanelHomeOnUSB(usbMount), "panel.json")
}

// UsesLegacyDataPaths reports paths still pointing at router /data storage.
func UsesLegacyDataPaths(cfg PanelConfig) bool {
	for _, p := range []string{
		cfg.Paths.StartupScript,
		cfg.Paths.IptablesScript,
		cfg.Paths.PanelDataDir,
		cfg.Logs.Startup,
		cfg.Logs.Panel,
	} {
		if strings.HasPrefix(p, "/data/") {
			return true
		}
	}
	return false
}

// MigrateLegacyDataPaths moves panel runtime paths from /data to USB next to Xray.
func MigrateLegacyDataPaths(cfg *PanelConfig) bool {
	if !UsesLegacyDataPaths(*cfg) {
		return false
	}
	usb := USBMountFromXrayBin(cfg.Paths.XrayBin)
	if usb == "" {
		usb = DefaultUSBMount
	}
	cfg.Paths = PathsForUSB(usb)
	cfg.Logs = LogsForPanelHome(cfg.Paths.PanelDataDir)
	cfg.Normalize()
	return true
}
