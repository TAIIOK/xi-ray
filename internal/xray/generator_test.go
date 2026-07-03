package xray

import (
	"encoding/json"
	"testing"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

func TestGenerateSingle(t *testing.T) {
	cfg := config.DefaultPanelConfig()
	cfg.Nodes = []config.Node{{
		ID: "node-12345678", Name: "test", Protocol: "vless",
		Address: "example.com", Port: 443, UUID: "00000000-0000-0000-0000-000000000001",
		Security: "reality", Network: "tcp", Flow: "xtls-rprx-vision",
		SNI: "www.example.com", Fingerprint: "edge", PublicKey: "abc", ShortID: "1234",
	}}
	cfg.Selection = config.Selection{
		Mode:          "single",
		ActiveNodeIDs: []string{"node-12345678"},
		FallbackOrder: []string{"node-12345678"},
	}

	raw, err := Generate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["observatory"] == nil {
		t.Fatalf("observatory should be present when enabled in single mode")
	}
	assertOutboundTag(t, doc, "api")
	assertAPIServices(t, doc, "HandlerService", "StatsService", "ObservatoryService")
}

func TestGenerateSingleObservatoryDisabled(t *testing.T) {
	cfg := config.DefaultPanelConfig()
	cfg.Observatory.Enabled = false
	cfg.Nodes = []config.Node{{
		ID: "node-12345678", Name: "test", Protocol: "vless",
		Address: "example.com", Port: 443, UUID: "00000000-0000-0000-0000-000000000001",
		Security: "reality", Network: "tcp", Flow: "xtls-rprx-vision",
		SNI: "www.example.com", Fingerprint: "edge", PublicKey: "abc", ShortID: "1234",
	}}
	cfg.Selection = config.Selection{
		Mode:          "single",
		ActiveNodeIDs: []string{"node-12345678"},
		FallbackOrder: []string{"node-12345678"},
	}

	raw, err := Generate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["observatory"] != nil {
		t.Fatalf("observatory should be absent when disabled")
	}
	assertAPIServices(t, doc, "HandlerService", "StatsService")
	assertNotAPIService(t, doc, "ObservatoryService")
}

func assertOutboundTag(t *testing.T, doc map[string]any, tag string) {
	t.Helper()
	outbounds, ok := doc["outbounds"].([]any)
	if !ok {
		t.Fatal("missing outbounds")
	}
	for _, ob := range outbounds {
		m, ok := ob.(map[string]any)
		if !ok {
			continue
		}
		if m["tag"] == tag {
			return
		}
	}
	t.Fatalf("outbound tag %q not found", tag)
}

func assertAPIServices(t *testing.T, doc map[string]any, want ...string) {
	t.Helper()
	api, ok := doc["api"].(map[string]any)
	if !ok {
		t.Fatal("missing api section")
	}
	services, ok := api["services"].([]any)
	if !ok {
		t.Fatal("missing api.services")
	}
	got := map[string]struct{}{}
	for _, s := range services {
		got[s.(string)] = struct{}{}
	}
	for _, w := range want {
		if _, ok := got[w]; !ok {
			t.Fatalf("api.services missing %q", w)
		}
	}
}

func assertNotAPIService(t *testing.T, doc map[string]any, name string) {
	t.Helper()
	api := doc["api"].(map[string]any)
	for _, s := range api["services"].([]any) {
		if s.(string) == name {
			t.Fatalf("unexpected api service %q", name)
		}
	}
}

func TestGenerateMulti(t *testing.T) {
	cfg := config.DefaultPanelConfig()
	cfg.Nodes = []config.Node{
		{ID: "node-aaaaaaaa", Name: "a", Protocol: "vless", Address: "a.example.com", Port: 443, UUID: "00000000-0000-0000-0000-000000000001", Security: "tls", Network: "tcp", SNI: "a.example.com"},
		{ID: "node-bbbbbbbb", Name: "b", Protocol: "vless", Address: "b.example.com", Port: 443, UUID: "00000000-0000-0000-0000-000000000002", Security: "tls", Network: "tcp", SNI: "b.example.com"},
	}
	cfg.Selection = config.Selection{
		Mode:          "multi",
		ActiveNodeIDs: []string{"node-aaaaaaaa", "node-bbbbbbbb"},
		FallbackOrder: []string{"node-aaaaaaaa", "node-bbbbbbbb"},
	}
	raw, err := Generate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 100 {
		t.Fatal("config too short")
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	assertOutboundTag(t, doc, "api")
	assertAPIServices(t, doc, "HandlerService", "StatsService", "ObservatoryService")
}
