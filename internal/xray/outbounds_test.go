package xray

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

func TestGenerateVMessOutbound(t *testing.T) {
	cfg := config.DefaultPanelConfig()
	cfg.Iptables.TCPPort = 22346
	cfg.Nodes = []config.Node{{
		ID: "node-vmess01", Name: "vmess", Protocol: "vmess",
		Address: "vm.example.com", Port: 443, UUID: "00000000-0000-0000-0000-000000000001",
		Security: "tls", Network: "tcp", SNI: "vm.example.com",
	}}
	cfg.Selection = config.Selection{Mode: "single", ActiveNodeIDs: []string{"node-vmess01"}, FallbackOrder: []string{"node-vmess01"}}
	raw, err := Generate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"protocol": "vmess"`) {
		t.Fatal("expected vmess outbound")
	}
	if !strings.Contains(string(raw), `"port": 22346`) {
		t.Fatal("expected custom tcp port in inbounds")
	}
}

func TestGenerateTrojanOutbound(t *testing.T) {
	cfg := config.DefaultPanelConfig()
	cfg.Nodes = []config.Node{{
		ID: "node-trojan1", Name: "trojan", Protocol: "trojan",
		Address: "tr.example.com", Port: 443, Password: "pw", Security: "tls", SNI: "tr.example.com",
	}}
	cfg.Selection = config.Selection{Mode: "single", ActiveNodeIDs: []string{"node-trojan1"}, FallbackOrder: []string{"node-trojan1"}}
	raw, err := Generate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	outbounds := doc["outbounds"].([]any)
	found := false
	for _, ob := range outbounds {
		m := ob.(map[string]any)
		if m["protocol"] == "trojan" {
			found = true
		}
	}
	if !found {
		t.Fatal("trojan outbound missing")
	}
}

func TestGenerateIPTablesCustomPorts(t *testing.T) {
	cfg := config.DefaultPanelConfig()
	cfg.Iptables.TCPPort = 33346
	cfg.Iptables.UDPPort = 33345
	cfg.Iptables.GuestSubnet = "192.168.33.0/24"
	script, err := GenerateIPTablesScript(cfg, []config.Node{{Address: "1.2.3.4", Port: 443, Protocol: "vless", UUID: "x"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, "TCP_PORT=33346") || !strings.Contains(script, "UDP_PORT=33345") {
		t.Fatal("custom ports not in script")
	}
}
