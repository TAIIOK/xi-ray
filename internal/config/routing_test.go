package config

import "testing"

func TestValidateRouting(t *testing.T) {
	r := DefaultRouting()
	r.Rules = []RoutingRule{
		{ID: "1", Name: "bad", Action: "direct", Enabled: true},
	}
	if err := ValidateRouting(r); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestNormalizeRuleOrder(t *testing.T) {
	r := Routing{RuleOrder: []string{"block", "direct"}}
	r.Normalize()
	if len(r.RuleOrder) != 3 || r.RuleOrder[0] != "block" || r.RuleOrder[1] != "direct" || r.RuleOrder[2] != "proxy" {
		t.Fatalf("order: %v", r.RuleOrder)
	}
}

func TestRoutingIsEmptyPayload(t *testing.T) {
	empty := Routing{}
	if !empty.IsEmptyPayload() {
		t.Fatal("expected empty routing payload")
	}
	explicit := DefaultRouting()
	explicit.Rules = []RoutingRule{}
	if explicit.IsEmptyPayload() {
		t.Fatal("fully populated defaults should not be empty payload")
	}
	withRules := Routing{Rules: []RoutingRule{{ID: "1", Name: "x", Action: "direct", Domains: []string{"geosite:cn"}, Enabled: true}}}
	if withRules.IsEmptyPayload() {
		t.Fatal("routing with rules is not empty payload")
	}
}