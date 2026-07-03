package xray

import (
	"github.com/taiiok/xiaomi-vless/internal/config"
)

var guestInbounds = []string{"redirect-in", "tproxy-in", "socks-in"}

func BuildRouting(cfg config.PanelConfig, proxyTags []string, nodes []config.Node) map[string]any {
	r := cfg.Routing
	r.Normalize()

	useBalancer := cfg.Selection.Mode == "multi" && len(proxyTags) > 1 && cfg.Observatory.Enabled

	rules := []map[string]any{
		{"type": "field", "inboundTag": []string{"api"}, "outboundTag": "api"},
	}

	if r.BypassPrivate {
		rules = append(rules, map[string]any{
			"type": "field", "ip": []string{"geoip:private"}, "outboundTag": "direct",
		})
	}

	if r.BypassVPNHosts {
		for _, node := range nodes {
			if node.Address == "" {
				continue
			}
			rules = append(rules, map[string]any{
				"type": "field", "domain": []string{node.Address}, "outboundTag": "direct",
			})
		}
	}

	for _, action := range r.RuleOrder {
		for _, rule := range r.Rules {
			if !rule.Enabled || rule.Action != action {
				continue
			}
			if xrule := userRuleToXray(rule, useBalancer, proxyTags); xrule != nil {
				rules = append(rules, xrule)
			}
		}
	}

	guestRule := map[string]any{
		"type":       "field",
		"inboundTag": guestInbounds,
	}
	applyAction(guestRule, r.DefaultGuest, useBalancer, proxyTags)
	rules = append(rules, guestRule)

	result := map[string]any{
		"domainStrategy": r.DomainStrategy,
		"rules":          rules,
	}
	if useBalancer {
		result["balancers"] = []map[string]any{{
			"tag":      "guest-balancer",
			"selector": []string{"proxy-"},
			"strategy": map[string]any{"type": "leastPing"},
		}}
	}
	return result
}

func userRuleToXray(rule config.RoutingRule, useBalancer bool, proxyTags []string) map[string]any {
	if len(rule.Domains) == 0 && len(rule.IPs) == 0 {
		return nil
	}
	xrule := map[string]any{"type": "field"}
	if len(rule.Domains) > 0 {
		xrule["domain"] = rule.Domains
	}
	if len(rule.IPs) > 0 {
		xrule["ip"] = rule.IPs
	}
	applyAction(xrule, rule.Action, useBalancer, proxyTags)
	return xrule
}

func applyAction(rule map[string]any, action string, useBalancer bool, proxyTags []string) {
	switch action {
	case "direct":
		rule["outboundTag"] = "direct"
	case "block":
		rule["outboundTag"] = "block"
	default:
		if useBalancer {
			rule["balancerTag"] = "guest-balancer"
		} else if len(proxyTags) > 0 {
			rule["outboundTag"] = proxyTags[0]
		} else {
			rule["outboundTag"] = "direct"
		}
	}
}
