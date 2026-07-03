package setup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathsFromUSBMount(t *testing.T) {
	p := PathsFromUSBMount("/mnt/usb-abc123")
	home := "/mnt/usb-abc123/xiaomi-vless"
	if p.XrayBin != "/mnt/usb-abc123/xray/bin/xray" {
		t.Fatalf("xray bin: %s", p.XrayBin)
	}
	if p.XrayConfig != "/mnt/usb-abc123/xray/config.json" {
		t.Fatalf("config: %s", p.XrayConfig)
	}
	if p.StartupScript != home+"/startup_xray_guest.sh" {
		t.Fatalf("startup: %s", p.StartupScript)
	}
	if p.PanelDataDir != home {
		t.Fatalf("panel home: %s", p.PanelDataDir)
	}
}

func TestUSBPathFromXrayBin(t *testing.T) {
	got := usbPathFromXrayBin("/mnt/usb-ed49605f/xray/bin/xray")
	if got != "/mnt/usb-ed49605f" {
		t.Fatalf("got %s", got)
	}
}

func TestVerifyPathsEmpty(t *testing.T) {
	checks := VerifyPaths(PathsFromUSBMount(""))
	checks[0].Path = ""
	if checks[0].OK {
		t.Fatal("expected xray_bin not ok")
	}
}

func TestScriptEmbed(t *testing.T) {
	for _, name := range []string{"startup_xray_guest.sh", "xray-guest-iptables.sh", "xray-guest-sysctl.sh"} {
		data, err := Script(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(data) < 50 {
			t.Fatalf("%s too short", name)
		}
	}
}

func TestPathWritable(t *testing.T) {
	dir := t.TempDir()
	if !pathWritable(dir) {
		t.Fatal("temp dir should be writable")
	}
	_ = filepath.Join(dir, "sub")
	_ = os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
}
