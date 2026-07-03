package xray

import "testing"

func TestStagingConfigPath(t *testing.T) {
	got := StagingConfigPath("/mnt/usb-ed49605f/xiaomi-vless", "/mnt/usb-ed49605f/xray/config.json")
	want := "/mnt/usb-ed49605f/xiaomi-vless/config.staging.json"
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestXrayAssetDir(t *testing.T) {
	got := XrayAssetDir("/mnt/usb-ed49605f/xray/bin/xray")
	if got != "/mnt/usb-ed49605f/xray" {
		t.Fatalf("got %s", got)
	}
}
