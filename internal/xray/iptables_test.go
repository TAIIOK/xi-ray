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
