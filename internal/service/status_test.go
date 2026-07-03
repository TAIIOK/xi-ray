package service

import (
	"testing"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

func TestApplySOCKSProbeFallback(t *testing.T) {
	cfg := config.DefaultPanelConfig()
	cfg.Nodes = []config.Node{{
		ID: "1bcaadad-xxxx", Name: "test", LastHealth: "",
	}}
	cfg.Selection.ActiveNodeIDs = []string{"1bcaadad-xxxx"}

	obs := ObservatoryStatus{
		Nodes: []NodeHealth{{
			ID: "1bcaadad-xxxx", Name: "test", Tag: "proxy-1bcaadad",
			Health: "unknown", Source: "cache",
		}},
	}

	out := applySOCKSProbeFallback(obs, cfg, 294)
	if out.ActiveOutbound != "proxy-1bcaadad" {
		t.Fatalf("active outbound = %q", out.ActiveOutbound)
	}
	if out.Nodes[0].Health != "ok" || out.Nodes[0].Source != "socks-probe" {
		t.Fatalf("node health not updated: %+v", out.Nodes[0])
	}
	if out.Nodes[0].LatencyMs != 294 {
		t.Fatalf("latency = %d", out.Nodes[0].LatencyMs)
	}
}

func TestPrimaryProxyTag(t *testing.T) {
	cfg := config.DefaultPanelConfig()
	cfg.Nodes = []config.Node{{ID: "abc12345-zzzz", Name: "n1"}}
	cfg.Selection.ActiveNodeIDs = []string{"abc12345-zzzz"}
	if got := primaryProxyTag(cfg); got != "proxy-abc12345" {
		t.Fatalf("got %q", got)
	}
}
