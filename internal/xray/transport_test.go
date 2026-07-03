package xray

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

func TestGenerateXHTTPOutbound(t *testing.T) {
	cfg := config.DefaultPanelConfig()
	cfg.Nodes = []config.Node{{
		ID: "node-xhttp01", Name: "xhttp", Protocol: "vless",
		Address: "cdn.example.com", Port: 443, UUID: "00000000-0000-0000-0000-000000000001",
		Security: "tls", Network: "xhttp", SNI: "cdn.example.com", Fingerprint: "chrome",
		Path: "/tunnel", Host: "cdn.example.com", XHTTPMode: "auto",
		XHTTPExtra: map[string]any{"xPaddingBytes": "100-1000"},
	}}
	cfg.Selection = config.Selection{
		Mode: "single", ActiveNodeIDs: []string{"node-xhttp01"}, FallbackOrder: []string{"node-xhttp01"},
	}

	raw, err := Generate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"network": "xhttp"`) {
		t.Fatal("expected xhttp network")
	}
	if !strings.Contains(string(raw), `"xhttpSettings"`) {
		t.Fatal("missing xhttpSettings")
	}
	if !strings.Contains(string(raw), `"/tunnel"`) {
		t.Fatal("missing path")
	}

	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for _, ob := range doc["outbounds"].([]any) {
		m := ob.(map[string]any)
		if m["tag"] != "proxy-node-xht" {
			continue
		}
		stream := m["streamSettings"].(map[string]any)
		xhttp := stream["xhttpSettings"].(map[string]any)
		if xhttp["mode"] != "auto" {
			t.Fatalf("mode: %v", xhttp["mode"])
		}
		if xhttp["path"] != "/tunnel" {
			t.Fatalf("path: %v", xhttp["path"])
		}
		return
	}
	t.Fatal("proxy outbound not found")
}

func TestNormalizeNetworkSplitHTTP(t *testing.T) {
	if got := normalizeNetwork("splithttp"); got != "xhttp" {
		t.Fatalf("got %q", got)
	}
}
