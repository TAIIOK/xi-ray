package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

type PathCheck struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Exists  bool   `json:"exists"`
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	Hint    string `json:"hint,omitempty"`
}

type USBMount struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	HasXray bool   `json:"has_xray"`
	XrayBin string `json:"xray_bin,omitempty"`
}

type SetupAction struct {
	Action  string `json:"action"`
	Path    string `json:"path"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type SetupResult struct {
	OK      bool          `json:"ok"`
	Message string        `json:"message"`
	Actions []SetupAction `json:"actions"`
	Checks  []PathCheck   `json:"checks"`
}

func VerifyPaths(p config.Paths) []PathCheck {
	_, geoip, geosite := geoAssetDestinations(p.XrayBin)
	return []PathCheck{
		verifyFile("xray_bin", "Xray binary", p.XrayBin, true),
		verifyFileOrParent("xray_config", "Xray config", p.XrayConfig, false),
		verifyFile("geoip_dat", "geoip.dat", geoip, false),
		verifyFile("geosite_dat", "geosite.dat", geosite, false),
		verifyFile("startup_script", "Startup script", p.StartupScript, true),
		verifyFile("iptables_script", "iptables script", p.IptablesScript, true),
		verifyDir("panel_data_dir", "Panel data dir", p.PanelDataDir),
	}
}

func AllChecksOK(checks []PathCheck) bool {
	for _, c := range checks {
		if !c.OK {
			return false
		}
	}
	return true
}

func DiscoverUSBMounts() []USBMount {
	var mounts []USBMount
	entries, err := os.ReadDir("/mnt")
	if err != nil {
		return mounts
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		mountPath := filepath.Join("/mnt", e.Name())
		xrayBin := filepath.Join(mountPath, "xray/bin/xray")
		hasXray := false
		if info, err := os.Stat(xrayBin); err == nil && !info.IsDir() {
			hasXray = true
		}
		mounts = append(mounts, USBMount{
			Path:    mountPath,
			Name:    e.Name(),
			HasXray: hasXray,
			XrayBin: xrayBin,
		})
	}
	return mounts
}

func PathsFromUSBMount(usbMount string) config.Paths {
	return config.PathsForUSB(usbMount)
}

func RunSetup(p config.Paths) SetupResult {
	result := SetupResult{Actions: []SetupAction{}}
	add := func(action, path, msg string, ok bool) {
		result.Actions = append(result.Actions, SetupAction{Action: action, Path: path, OK: ok, Message: msg})
	}

	if err := config.ValidatePaths(p); err != nil {
		result.Message = err.Error()
		result.Checks = VerifyPaths(p)
		return result
	}

	if err := config.EnsureDir(p.PanelDataDir, config.PanelDirPerm); err != nil {
		add("mkdir", p.PanelDataDir, err.Error(), false)
	} else {
		add("mkdir", p.PanelDataDir, "created or exists", true)
	}

	configDir := filepath.Dir(p.XrayConfig)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		add("mkdir", configDir, err.Error(), false)
	} else {
		add("mkdir", configDir, "xray config directory ready", true)
	}

	if _, err := os.Stat(p.XrayConfig); os.IsNotExist(err) {
		if err := os.WriteFile(p.XrayConfig, []byte("{}\n"), 0o644); err != nil {
			add("create", p.XrayConfig, err.Error(), false)
		} else {
			add("create", p.XrayConfig, "placeholder config created (Apply will overwrite)", true)
		}
	}

	for _, item := range []struct {
		dest, script string
	}{
		{p.StartupScript, "startup_xray_guest.sh"},
		{p.IptablesScript, "xray-guest-iptables.sh"},
		{SysctlPath(p), "xray-guest-sysctl.sh"},
		{filepath.Join(p.PanelDataDir, "boot-xiaomi-vless.sh"), "boot-xiaomi-vless.sh"},
	} {
		if err := installScript(item.dest, item.script); err != nil {
			add("install", item.dest, err.Error(), false)
		} else {
			add("install", item.dest, "script installed", true)
		}
	}

	if err := writeXrayEnv(p); err != nil {
		add("env", filepath.Join(p.PanelDataDir, "xray.env"), err.Error(), false)
	} else {
		add("env", filepath.Join(p.PanelDataDir, "xray.env"), "environment file written", true)
	}

	if err := ensureAutostart(p.PanelDataDir); err != nil {
		add("autostart", "/data/startup_user.sh", err.Error(), false)
	} else {
		add("autostart", "/data/startup_user.sh", "boot hook registered", true)
	}

	result.Checks = VerifyPaths(p)
	result.OK = AllChecksOK(result.Checks)
	if result.OK {
		result.Message = "setup completed, all checks passed"
	} else {
		result.Message = "setup finished with warnings — check paths (USB must be mounted with xray)"
	}
	return result
}

func installScript(dest, scriptName string) error {
	if info, err := os.Stat(dest); err == nil && info.Size() > 100 {
		return os.Chmod(dest, 0o755)
	}
	data, err := Script(scriptName)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(dest, data, 0o755); err != nil {
		return err
	}
	return nil
}

func writeXrayEnv(p config.Paths) error {
	usbPath := usbPathFromXrayBin(p.XrayBin)
	home := p.PanelDataDir
	sysctl := SysctlPath(p)
	logPath := filepath.Join(home, "xray-startup.log")
	binDir := XrayBinDir(p.XrayBin)
	content := fmt.Sprintf(`PANEL_HOME=%q
USB_PATH=%q
XRAY=%q
CONFIG=%q
XRAY_LOCATION_ASSET=%q
IPTABLES=%q
SYSCTL=%q
LOG=%q
`, home, usbPath, p.XrayBin, p.XrayConfig, binDir, p.IptablesScript, sysctl, logPath)
	envPath := filepath.Join(home, "xray.env")
	if err := config.EnsureDir(home, config.PanelDirPerm); err != nil {
		return err
	}
	return os.WriteFile(envPath, []byte(content), config.ConfigFilePerm)
}

func usbPathFromXrayBin(xrayBin string) string {
	return config.USBMountFromXrayBin(xrayBin)
}

func ensureAutostart(panelHome string) error {
	const marker = "# xiaomi-vless-boot"
	userStartup := "/data/startup_user.sh"
	bootDst := "/data/xiaomi-vless-boot.sh"
	bootSrc := filepath.Join(panelHome, "boot-xiaomi-vless.sh")
	line := fmt.Sprintf("[ -x %s ] && %s >/dev/null 2>&1 &", bootDst, bootDst)

	if data, err := os.ReadFile(bootSrc); err == nil {
		if err := os.WriteFile(bootDst, data, 0o755); err != nil {
			return fmt.Errorf("install boot script: %w", err)
		}
	}

	if data, err := os.ReadFile(userStartup); err == nil {
		if strings.Contains(string(data), marker) {
			return nil
		}
	} else if os.IsNotExist(err) {
		if err := os.WriteFile(userStartup, []byte("#!/bin/sh\n"), 0o755); err != nil {
			return err
		}
	} else {
		return err
	}

	f, err := os.OpenFile(userStartup, os.O_APPEND|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "\n%s\n%s\n", marker, line)
	return err
}

func verifyFile(key, name, path string, mustExecutable bool) PathCheck {
	c := PathCheck{Key: key, Name: name, Path: path}
	if path == "" {
		c.Message = "не задан"
		c.Hint = "укажите путь"
		return c
	}
	info, err := os.Stat(path)
	if err != nil {
		c.Message = "не найден"
		c.Hint = hintForMissing(key)
		if key == "startup_script" || key == "iptables_script" {
			if pathWritable(filepath.Dir(path)) {
				c.Hint = "нажмите «Установить недостающее» — панель скопирует скрипт сюда"
			}
		}
		return c
	}
	if info.IsDir() {
		c.Message = "это каталог, нужен файл"
		return c
	}
	c.Exists = true
	if mustExecutable {
		if info.Mode()&0o111 == 0 {
			c.Message = "файл не исполняемый"
			c.Hint = "chmod +x или переустановите"
			return c
		}
	}
	c.OK = true
	c.Message = "OK"
	return c
}

func verifyFileOrParent(key, name, path string, mustExecutable bool) PathCheck {
	c := verifyFile(key, name, path, mustExecutable)
	if c.OK {
		return c
	}
	parent := filepath.Dir(path)
	if parent == "" || parent == "." {
		return c
	}
	if info, err := os.Stat(parent); err == nil && info.IsDir() {
		c.Exists = true
		if pathWritable(parent) {
			c.OK = true
			c.Message = "файл будет создан при Apply"
			c.Hint = ""
		} else {
			c.Message = "каталог есть, но config отсутствует"
			c.Hint = "Apply создаст config или нажмите «Установить»"
		}
	}
	return c
}

func verifyDir(key, name, path string) PathCheck {
	c := PathCheck{Key: key, Name: name, Path: path}
	if path == "" {
		c.Message = "не задан"
		return c
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) && pathWritable(filepath.Dir(path)) {
			c.OK = true
			c.Message = "будет создан"
			return c
		}
		c.Message = "не найден"
		c.Hint = "нажмите «Установить недостающее»"
		return c
	}
	if !info.IsDir() {
		c.Message = "не каталог"
		return c
	}
	c.Exists = true
	c.OK = true
	c.Message = "OK"
	return c
}

func pathWritable(dir string) bool {
	return PathWritable(dir)
}

// PathWritable reports whether the current user can write into dir.
func PathWritable(dir string) bool {
	if dir == "" {
		return false
	}
	test := filepath.Join(dir, ".panel-write-test")
	if err := os.WriteFile(test, []byte("x"), 0o600); err != nil {
		return false
	}
	_ = os.Remove(test)
	return true
}

func XrayVersion(path string) string {
	if path == "" {
		return ""
	}
	out, err := exec.Command(path, "version").Output()
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(strings.Split(string(out), "\n")[0])
	if len(line) > 80 {
		return line[:80]
	}
	return line
}
