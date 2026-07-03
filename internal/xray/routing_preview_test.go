package xray

import (
	"testing"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

func TestBuildRoutingPreview(t *testing.T) {
	cfg := config.DefaultPanelConfig()
	cfg.Routing.Rules = []config.RoutingRule{
		{ID: "1", Name: "CN", Action: "direct", Domains: []string{"geosite:cn"}, Enabled: true},
	}
	cfg.Nodes = []config.Node{{
		ID: "node-12345678", Protocol: "vless", Address: "vpn.example.com", Port: 443,
		UUID: "00000000-0000-0000-0000-000000000001", Security: "tls", Network: "tcp", Name: "vpn",
	}}
	cfg.Selection = config.Selection{
		Mode: "single", ActiveNodeIDs: []string{"node-12345678"}, FallbackOrder: []string{"node-12345678"},
	}

	preview := BuildRoutingPreview(cfg, []string{"proxy-node-123"}, cfg.Nodes)
	if len(preview) < 4 {
		t.Fatalf("preview too short: %d", len(preview))
	}
	if preview[len(preview)-1].Kind != "default" {
		t.Fatalf("last rule should be default")
	}
}
