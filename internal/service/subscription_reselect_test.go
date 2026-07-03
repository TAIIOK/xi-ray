package service

import (
	"testing"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

func TestReselectAfterSubscriptionUpdateFirst(t *testing.T) {
	cfg := &config.PanelConfig{
		Selection: config.Selection{Mode: "single", ActiveNodeIDs: []string{"old-id"}},
		Nodes: []config.Node{
			{ID: "n1", Protocol: "vless", Name: "a", Hash: "h1", SubscriptionID: "sub1"},
			{ID: "n2", Protocol: "vless", Name: "b", Hash: "h2", SubscriptionID: "sub1"},
		},
	}
	changed := reselectAfterSubscriptionUpdate(cfg, "sub1", map[string]struct{}{"old": {}}, "first")
	if !changed {
		t.Fatal("expected selection change")
	}
	if cfg.Selection.ActiveNodeIDs[0] != "n1" {
		t.Fatalf("got %v", cfg.Selection.ActiveNodeIDs)
	}
}

func TestReselectKeepWhenValid(t *testing.T) {
	cfg := &config.PanelConfig{
		Selection: config.Selection{Mode: "single", ActiveNodeIDs: []string{"n1"}, FallbackOrder: []string{"n1"}},
		Nodes: []config.Node{
			{ID: "n1", Protocol: "vless", Hash: "h1", SubscriptionID: "sub1"},
			{ID: "n2", Protocol: "vless", Hash: "h2", SubscriptionID: "sub1"},
		},
	}
	changed := reselectAfterSubscriptionUpdate(cfg, "sub1", map[string]struct{}{"h1": {}, "h2": {}}, "keep")
	if changed {
		t.Fatal("expected no change")
	}
	if cfg.Selection.ActiveNodeIDs[0] != "n1" {
		t.Fatalf("got %v", cfg.Selection.ActiveNodeIDs)
	}
}

func TestHashSetsEqual(t *testing.T) {
	a := map[string]struct{}{"x": {}, "y": {}}
	b := map[string]struct{}{"y": {}, "x": {}}
	if !hashSetsEqual(a, b) {
		t.Fatal("expected equal")
	}
}
