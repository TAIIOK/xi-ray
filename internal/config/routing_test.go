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
