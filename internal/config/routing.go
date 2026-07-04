package config

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type Routing struct {
	DomainStrategy string        `json:"domain_strategy"`
	RuleOrder      []string      `json:"rule_order"`
	DefaultGuest   string        `json:"default_guest_action"`
	BypassPrivate  bool          `json:"bypass_private"`
	BypassVPNHosts bool          `json:"bypass_vpn_hosts"`
	Rules          []RoutingRule `json:"rules"`
}

type RoutingRule struct {
	ID      string   `json:"id"`
	Name    string   `json:"name,omitempty"`
	Action  string   `json:"action"`
	Domains []string `json:"domains,omitempty"`
	IPs     []string `json:"ips,omitempty"`
	Enabled bool     `json:"enabled"`
}

func DefaultRouting() Routing {
	return Routing{
		DomainStrategy: "IPIfNonMatch",
		RuleOrder:      []string{"direct", "proxy", "block"},
		DefaultGuest:   "proxy",
		BypassPrivate:  true,
		BypassVPNHosts: true,
		Rules:          []RoutingRule{},
	}
}

// IsEmptyPayload reports whether routing was omitted or sent as {} from the client.
// Normalize() fills defaults and must not run before this check when deciding to skip an update.
func (r Routing) IsEmptyPayload() bool {
	return r.DomainStrategy == "" &&
		len(r.RuleOrder) == 0 &&
		r.DefaultGuest == "" &&
		len(r.Rules) == 0 &&
		!r.BypassPrivate &&
		!r.BypassVPNHosts
}

func (r *Routing) Normalize() {
	if r.DomainStrategy == "" {
		r.DomainStrategy = "IPIfNonMatch"
	}
	if r.DefaultGuest == "" {
		r.DefaultGuest = "proxy"
	}
	r.RuleOrder = normalizeRuleOrder(r.RuleOrder)
	for i := range r.Rules {
		if r.Rules[i].ID == "" {
			r.Rules[i].ID = uuid.NewString()
		}
		r.Rules[i].Action = strings.ToLower(strings.TrimSpace(r.Rules[i].Action))
		r.Rules[i].Domains = cleanMatchers(r.Rules[i].Domains)
		r.Rules[i].IPs = cleanMatchers(r.Rules[i].IPs)
		if r.Rules[i].Name == "" {
			r.Rules[i].Name = fmt.Sprintf("rule-%d", i+1)
		}
	}
}

func normalizeRuleOrder(order []string) []string {
	want := []string{"direct", "proxy", "block"}
	if len(order) == 0 {
		return want
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, 3)
	for _, a := range order {
		a = strings.ToLower(strings.TrimSpace(a))
		if a != "direct" && a != "proxy" && a != "block" {
			continue
		}
		if _, ok := seen[a]; ok {
			continue
		}
		seen[a] = struct{}{}
		out = append(out, a)
	}
	for _, a := range want {
		if _, ok := seen[a]; !ok {
			out = append(out, a)
		}
	}
	return out
}

func cleanMatchers(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func ValidateRouting(r Routing) error {
	r.Normalize()
	validStrategy := map[string]struct{}{
		"AsIs": {}, "IPIfNonMatch": {}, "IpOnDemand": {},
	}
	if _, ok := validStrategy[r.DomainStrategy]; !ok {
		return fmt.Errorf("invalid domain_strategy: %s", r.DomainStrategy)
	}
	validAction := map[string]struct{}{"direct": {}, "proxy": {}, "block": {}}
	if _, ok := validAction[r.DefaultGuest]; !ok {
		return fmt.Errorf("invalid default_guest_action: %s", r.DefaultGuest)
	}
	for _, rule := range r.Rules {
		if !rule.Enabled {
			continue
		}
		if _, ok := validAction[rule.Action]; !ok {
			return fmt.Errorf("rule %q: invalid action %s", rule.Name, rule.Action)
		}
		if len(rule.Domains) == 0 && len(rule.IPs) == 0 {
			return fmt.Errorf("rule %q: domains or ips required", rule.Name)
		}
	}
	return nil
}
