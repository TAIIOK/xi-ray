package xray

import (
	"encoding/json"
	"testing"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

func TestBuildRoutingRuleOrder(t *testing.T) {
	cfg := config.DefaultPanelConfig()
	cfg.Routing = config.Routing{
		DomainStrategy: "IPIfNonMatch",
		RuleOrder:      []string{"direct", "proxy", "block"},
		DefaultGuest:   "proxy",
		BypassPrivate:  true,
		BypassVPNHosts: false,
		Rules: []config.RoutingRule{
			{ID: "1", Name: "cn", Action: "direct", Domains: []string{"geosite:cn"}, Enabled: true},
			{ID: "2", Name: "ads", Action: "block", Domains: []string{"geosite:category-ads-all"}, Enabled: true},
			{ID: "3", Name: "ru-ip", Action: "proxy", IPs: []string{"geoip:ru"}, Enabled: true},
		},
	}
	cfg.Nodes = []config.Node{{
		ID: "node-12345678", Name: "test", Protocol: "vless",
		Address: "vpn.example.com", Port: 443, UUID: "00000000-0000-0000-0000-000000000001",
		Security: "tls", Network: "tcp", SNI: "vpn.example.com",
	}}
	cfg.Selection = config.Selection{
		Mode: "single", ActiveNodeIDs: []string{"node-12345678"}, FallbackOrder: []string{"node-12345678"},
	}

	raw, err := Generate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	rules := doc["routing"].(map[string]any)["rules"].([]any)

	directIdx, proxyIdx, blockIdx := -1, -1, -1
	for i, r := range rules {
		m := r.(map[string]any)
		if doms, ok := m["domain"].([]any); ok {
			for _, d := range doms {
				if d.(string) == "geosite:cn" {
					directIdx = i
				}
				if d.(string) == "geosite:category-ads-all" {
					blockIdx = i
				}
			}
		}
		if ips, ok := m["ip"].([]any); ok {
			for _, ip := range ips {
				if ip.(string) == "geoip:ru" {
					proxyIdx = i
				}
			}
		}
	}
	if directIdx < 0 || proxyIdx < 0 || blockIdx < 0 {
		t.Fatalf("missing rules: direct=%d proxy=%d block=%d", directIdx, proxyIdx, blockIdx)
	}
	if !(directIdx < proxyIdx && proxyIdx < blockIdx) {
		t.Fatalf("wrong order: direct=%d proxy=%d block=%d", directIdx, proxyIdx, blockIdx)
	}
}

func TestBuildRoutingDefaultGuestDirect(t *testing.T) {
	cfg := config.DefaultPanelConfig()
	cfg.Routing.DefaultGuest = "direct"
	cfg.Nodes = []config.Node{{
		ID: "node-12345678", Protocol: "vless", Address: "a.com", Port: 443,
		UUID: "00000000-0000-0000-0000-000000000001", Security: "tls", Network: "tcp",
	}}
	cfg.Selection = config.Selection{
		Mode: "single", ActiveNodeIDs: []string{"node-12345678"}, FallbackOrder: []string{"node-12345678"},
	}
	raw, err := Generate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !containsStr(string(raw), `"outboundTag": "direct"`) {
		t.Fatal("expected direct guest catch-all")
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexStr(s, sub) >= 0)
}

func indexStr(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
