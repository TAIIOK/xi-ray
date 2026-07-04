package xray

import (
	"strings"
	"testing"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

func TestGenerateIPTablesScriptIncludesHosts(t *testing.T) {
	cfg := config.DefaultPanelConfig()
	nodes := []config.Node{
		{Address: "vpn.example.com", Port: 443},
		{Address: "10.0.0.5", Port: 443},
	}
	script, err := GenerateIPTablesScript(cfg, nodes)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, "10.0.0.5") {
		t.Fatal("expected static ip bypass")
	}
	if !strings.Contains(script, "resolve_host") && !strings.Contains(script, "VPN_IPS") {
		t.Fatal("expected vpn bypass section")
	}
	if !strings.Contains(script, "XRAY_GUEST_TCP") {
		t.Fatal("expected tcp chain")
	}
}

func TestGenerateIPTablesTeardownScript(t *testing.T) {
	script := GenerateIPTablesTeardownScript()
	for _, want := range []string{
		"XRAY_GUEST_TCP",
		"XRAY_GUEST_UDP",
		"ip rule del fwmark 0x1 table 100",
		"ip route flush table 100",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("teardown script missing %q", want)
		}
	}
}

func TestIPTablesTeardownScriptPath(t *testing.T) {
	got := IPTablesTeardownScriptPath("/mnt/usb/xiaomi-vless/xray-guest-iptables.sh")
	want := "/mnt/usb/xiaomi-vless/xray-guest-iptables-down.sh"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
