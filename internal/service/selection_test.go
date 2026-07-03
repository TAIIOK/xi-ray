package service

import (
	"testing"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

func TestSanitizeSelectionDropsStaleIDs(t *testing.T) {
	cfg := &config.PanelConfig{
		Nodes: []config.Node{
			{ID: "real-node", Protocol: "vless", Name: "server"},
		},
		Selection: config.Selection{
			Mode:          "single",
			ActiveNodeIDs: []string{"85494449-2239-4e0b-bc22-f94edd708f68"},
			FallbackOrder: []string{"85494449-2239-4e0b-bc22-f94edd708f68"},
		},
	}
	sanitizeSelection(cfg)
	if len(cfg.Selection.ActiveNodeIDs) != 1 || cfg.Selection.ActiveNodeIDs[0] != "real-node" {
		t.Fatalf("active: %v", cfg.Selection.ActiveNodeIDs)
	}
	if cfg.Selection.FallbackOrder[0] != "real-node" {
		t.Fatalf("fallback: %v", cfg.Selection.FallbackOrder)
	}
}

func TestSanitizeSelectionKeepsValidIDs(t *testing.T) {
	cfg := &config.PanelConfig{
		Nodes: []config.Node{
			{ID: "node-a", Protocol: "vless"},
			{ID: "node-b", Protocol: "vmess"},
		},
		Selection: config.Selection{
			Mode:          "single",
			ActiveNodeIDs: []string{"node-a"},
			FallbackOrder: []string{"node-a"},
		},
	}
	sanitizeSelection(cfg)
	if cfg.Selection.ActiveNodeIDs[0] != "node-a" {
		t.Fatalf("unexpected change: %v", cfg.Selection.ActiveNodeIDs)
	}
}
