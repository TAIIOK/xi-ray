package xray

import "github.com/taiiok/xiaomi-vless/internal/config"

type PreviewRule struct {
	Index    int      `json:"index"`
	Kind     string   `json:"kind"`
	Name     string   `json:"name"`
	Action   string   `json:"action"`
	Domains  []string `json:"domains,omitempty"`
	IPs      []string `json:"ips,omitempty"`
	Outbound string   `json:"outbound"`
	Inbound  string   `json:"inbound,omitempty"`
}

func BuildRoutingPreview(cfg config.PanelConfig, proxyTags []string, nodes []config.Node) []PreviewRule {
	r := cfg.Routing
	r.Normalize()
	useBalancer := cfg.Selection.Mode == "multi" && len(proxyTags) > 1 && cfg.Observatory.Enabled

	var out []PreviewRule
	step := 1
	add := func(kind, name, action, outbound, inbound string, domains, ips []string) {
		out = append(out, PreviewRule{
			Index: step, Kind: kind, Name: name, Action: action,
			Domains: domains, IPs: ips, Outbound: outbound, Inbound: inbound,
		})
		step++
	}

	add("system", "Xray API", "direct", "api", "api", nil, nil)

	if r.BypassPrivate {
		add("system", "Private networks", "direct", "direct", "", nil, []string{"geoip:private"})
	}

	if r.BypassVPNHosts {
		for _, node := range nodes {
			if node.Address == "" {
				continue
			}
			add("system", "VPN host: "+node.Name, "direct", "direct", "", []string{node.Address}, nil)
		}
	}

	for _, action := range r.RuleOrder {
		for _, rule := range r.Rules {
			if !rule.Enabled || rule.Action != action {
				continue
			}
			target := outboundLabel(action, useBalancer, proxyTags)
			add("user", rule.Name, action, target, "", rule.Domains, rule.IPs)
		}
	}

	defaultTarget := outboundLabel(r.DefaultGuest, useBalancer, proxyTags)
	add("default", "Guest Wi‑Fi (остальной трафик)", r.DefaultGuest, defaultTarget, "redirect-in, tproxy-in, socks-in", nil, nil)

	return out
}

func outboundLabel(action string, useBalancer bool, proxyTags []string) string {
	switch action {
	case "direct":
		return "direct"
	case "block":
		return "block"
	default:
		if useBalancer {
			return "balancer:guest-balancer"
		}
		if len(proxyTags) > 0 {
			return proxyTags[0]
		}
		return "direct"
	}
}
