package service

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/taiiok/xiaomi-vless/internal/config"
	"github.com/taiiok/xiaomi-vless/internal/xray"
)

func testPanelWithNode(t *testing.T) (*config.Store, *PanelService) {
	t.Helper()
	dir := t.TempDir()
	store, err := config.NewStore(filepath.Join(dir, "panel.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetPassword("admin", "secret"); err != nil {
		t.Fatal(err)
	}
	nodeID := "node-12345678"
	if err := store.Update(func(cfg *config.PanelConfig) error {
		cfg.Setup.OnboardingCompleted = true
		cfg.Nodes = []config.Node{{
			ID: nodeID, Name: "test", Protocol: "vless",
			Address: "vpn.example.com", Port: 443,
			UUID: "00000000-0000-0000-0000-000000000001",
			Security: "tls", Network: "tcp", SNI: "vpn.example.com",
		}}
		cfg.Selection = config.Selection{
			Mode:          "single",
			ActiveNodeIDs: []string{nodeID},
			FallbackOrder: []string{nodeID},
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return store, NewPanelService(store)
}

func sampleRouting() config.Routing {
	return config.Routing{
		DomainStrategy: "IPIfNonMatch",
		RuleOrder:      []string{"direct", "block", "proxy"},
		DefaultGuest:   "proxy",
		BypassPrivate:  true,
		BypassVPNHosts: true,
		Rules: []config.RoutingRule{
			{ID: "cn", Name: "CN sites", Action: "direct", Domains: []string{"geosite:cn"}, Enabled: true},
			{ID: "ads", Name: "Block ads", Action: "block", Domains: []string{"geosite:category-ads-all"}, Enabled: true},
		},
	}
}

func TestUpdateRoutingPersists(t *testing.T) {
	store, panel := testPanelWithNode(t)
	want := sampleRouting()

	if err := panel.UpdateRouting(want); err != nil {
		t.Fatal(err)
	}
	got := store.Get().Routing
	if got.DefaultGuest != "proxy" {
		t.Fatalf("default_guest: got %q", got.DefaultGuest)
	}
	if len(got.Rules) != 2 || got.Rules[0].Domains[0] != "geosite:cn" {
		t.Fatalf("rules: %+v", got.Rules)
	}
	if got.RuleOrder[0] != "direct" || got.RuleOrder[1] != "block" {
		t.Fatalf("rule_order: %v", got.RuleOrder)
	}
}

func TestUpdateRoutingValidation(t *testing.T) {
	_, panel := testPanelWithNode(t)
	bad := config.DefaultRouting()
	bad.DomainStrategy = "invalid"
	if err := panel.UpdateRouting(bad); err == nil {
		t.Fatal("expected validation error for domain_strategy")
	}

	bad = config.DefaultRouting()
	bad.Rules = []config.RoutingRule{
		{ID: "1", Name: "empty", Action: "direct", Enabled: true},
	}
	if err := panel.UpdateRouting(bad); err == nil {
		t.Fatal("expected validation error for empty rule matchers")
	}
}

func TestGetRoutingPreview(t *testing.T) {
	_, panel := testPanelWithNode(t)
	if err := panel.UpdateRouting(sampleRouting()); err != nil {
		t.Fatal(err)
	}
	resp, err := panel.GetRouting()
	if err != nil {
		t.Fatal(err)
	}
	if resp.SelectionMode != "single" || resp.ActiveNodes != 1 {
		t.Fatalf("meta: mode=%s nodes=%d", resp.SelectionMode, resp.ActiveNodes)
	}
	if len(resp.Preview) < 4 {
		t.Fatalf("preview too short: %d", len(resp.Preview))
	}
	foundCN := false
	for _, row := range resp.Preview {
		if row.Name == "CN sites" && row.Action == "direct" {
			foundCN = true
		}
	}
	if !foundCN {
		t.Fatal("preview missing CN sites rule")
	}
}

func TestUpdateRoutingWithApplyNoApply(t *testing.T) {
	_, panel := testPanelWithNode(t)
	result, err := panel.UpdateRoutingWithApply(context.Background(), sampleRouting(), false)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Apply != nil {
		t.Fatalf("expected save-only result, got %+v", result)
	}
	if len(result.Preview) == 0 {
		t.Fatal("expected preview in result")
	}
}

func TestUpdateRoutingReflectedInXrayGenerate(t *testing.T) {
	store, panel := testPanelWithNode(t)
	want := sampleRouting()
	if err := panel.UpdateRouting(want); err != nil {
		t.Fatal(err)
	}

	raw, err := xray.Generate(store.Get())
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	rules := doc["routing"].(map[string]any)["rules"].([]any)

	directIdx, blockIdx := -1, -1
	for i, r := range rules {
		m := r.(map[string]any)
		if doms, ok := m["domain"].([]any); ok {
			for _, d := range doms {
				switch d.(string) {
				case "geosite:cn":
					directIdx = i
				case "geosite:category-ads-all":
					blockIdx = i
				}
			}
		}
	}
	if directIdx < 0 || blockIdx < 0 {
		t.Fatalf("missing user rules in xray config: direct=%d block=%d", directIdx, blockIdx)
	}
	if !(directIdx < blockIdx) {
		t.Fatalf("rule order not applied: direct=%d block=%d", directIdx, blockIdx)
	}
}

func TestUpdateSettingsPreservesRouting(t *testing.T) {
	store, panel := testPanelWithNode(t)
	custom := sampleRouting()
	if err := panel.UpdateRouting(custom); err != nil {
		t.Fatal(err)
	}
	cfg := store.Get()
	if err := panel.UpdateSettings(
		cfg.Paths, cfg.Network, cfg.Iptables, cfg.Observatory, cfg.Watchdog,
		cfg.FailOpen, cfg.SubscriptionsPolicy, cfg.Logs, custom,
	); err != nil {
		t.Fatal(err)
	}
	got := store.Get().Routing
	if len(got.Rules) != 2 || got.Rules[1].Action != "block" {
		t.Fatalf("routing lost after settings update: %+v", got.Rules)
	}
}

func TestUpdateSettingsIgnoresEmptyRoutingPayload(t *testing.T) {
	store, panel := testPanelWithNode(t)
	custom := sampleRouting()
	if err := panel.UpdateRouting(custom); err != nil {
		t.Fatal(err)
	}
	cfg := store.Get()
	empty := config.Routing{}
	if err := panel.UpdateSettings(
		cfg.Paths, cfg.Network, cfg.Iptables, cfg.Observatory, cfg.Watchdog,
		cfg.FailOpen, cfg.SubscriptionsPolicy, cfg.Logs, empty,
	); err != nil {
		t.Fatal(err)
	}
	got := store.Get().Routing
	if len(got.Rules) != 2 || got.Rules[0].Domains[0] != "geosite:cn" {
		t.Fatalf("empty routing payload must not overwrite existing rules: %+v", got.Rules)
	}
}

func TestUpdateSettingsCanChangeRouting(t *testing.T) {
	store, panel := testPanelWithNode(t)
	cfg := store.Get()
	updated := config.DefaultRouting()
	updated.DefaultGuest = "direct"
	updated.Rules = []config.RoutingRule{
		{ID: "ru", Name: "RU IP", Action: "direct", IPs: []string{"geoip:ru"}, Enabled: true},
	}
	if err := panel.UpdateSettings(
		cfg.Paths, cfg.Network, cfg.Iptables, cfg.Observatory, cfg.Watchdog,
		cfg.FailOpen, cfg.SubscriptionsPolicy, cfg.Logs, updated,
	); err != nil {
		t.Fatal(err)
	}
	got := store.Get().Routing
	if got.DefaultGuest != "direct" {
		t.Fatalf("default_guest: %q", got.DefaultGuest)
	}
	if len(got.Rules) != 1 || got.Rules[0].IPs[0] != "geoip:ru" {
		t.Fatalf("rules: %+v", got.Rules)
	}
}

func TestUpdateRoutingWithApplySavesEvenWhenApplyFails(t *testing.T) {
	store, panel := testPanelWithNode(t)
	want := sampleRouting()
	result, err := panel.UpdateRoutingWithApply(context.Background(), want, true)
	// Apply fails without real xray binary/paths on dev machine — routing must still persist.
	if err == nil && result != nil && result.OK {
		t.Skip("apply succeeded unexpectedly in test environment")
	}
	got := store.Get().Routing
	if len(got.Rules) != 2 || got.Rules[0].Domains[0] != "geosite:cn" {
		t.Fatalf("routing not persisted after apply attempt: %+v", got.Rules)
	}
}

func TestDisabledRoutingRuleExcludedFromXray(t *testing.T) {
	store, panel := testPanelWithNode(t)
	r := sampleRouting()
	r.Rules = []config.RoutingRule{
		{ID: "off", Name: "Disabled rule", Action: "direct", Domains: []string{"domain:disabled-routing-test.example"}, Enabled: false},
	}
	if err := panel.UpdateRouting(r); err != nil {
		t.Fatal(err)
	}
	raw, err := xray.Generate(store.Get())
	if err != nil {
		t.Fatal(err)
	}
	if contains(string(raw), "disabled-routing-test.example") {
		t.Fatal("disabled rule should not appear in xray config")
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
