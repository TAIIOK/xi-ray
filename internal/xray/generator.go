package xray

import (
	"encoding/json"
	"fmt"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

type GeneratedConfig struct {
	Log         map[string]any   `json:"log"`
	API         map[string]any   `json:"api"`
	Stats       map[string]any   `json:"stats"`
	DNS         map[string]any   `json:"dns"`
	Inbounds    []map[string]any `json:"inbounds"`
	Outbounds   []map[string]any `json:"outbounds"`
	Routing     map[string]any   `json:"routing"`
	Observatory map[string]any   `json:"observatory,omitempty"`
}

func Generate(cfg config.PanelConfig) ([]byte, error) {
	nodes, err := selectedNodes(cfg)
	if err != nil {
		return nil, err
	}

	outbounds := []map[string]any{
		{"tag": "direct", "protocol": "freedom"},
		{"tag": "block", "protocol": "blackhole"},
		{"tag": "api", "protocol": "freedom"},
	}

	var proxyTags []string
	for _, node := range nodes {
		tag := OutboundTag(node.ID)
		proxyTags = append(proxyTags, tag)
		ob, err := nodeToOutbound(tag, node)
		if err != nil {
			return nil, fmt.Errorf("node %s: %w", node.Name, err)
		}
		outbounds = append(outbounds, ob)
	}

	routing := BuildRouting(cfg, proxyTags, nodes)
	apiServices := []string{"HandlerService", "StatsService"}
	useObservatory := cfg.Observatory.Enabled && len(proxyTags) > 0
	if useObservatory {
		apiServices = append(apiServices, "ObservatoryService")
	}
	generated := GeneratedConfig{
		Log:   buildLog(cfg),
		API:   map[string]any{"tag": "api", "services": apiServices},
		Stats: map[string]any{},
		DNS: map[string]any{
			"servers": []any{
				map[string]any{"address": "8.8.8.8", "domains": []string{"geosite:cn"}},
				"https://dns.google/dns-query",
				"8.8.8.8",
			},
		},
		Inbounds:  baseInbounds(cfg),
		Outbounds: outbounds,
		Routing:   routing,
	}

	if useObservatory {
		generated.Observatory = map[string]any{
			"subjectSelector":   []string{"proxy-"},
			"probeUrl":          cfg.Observatory.ProbeURL,
			"probeInterval":     cfg.Observatory.ProbeInterval,
			"enableConcurrency": true,
		}
	}

	return json.MarshalIndent(generated, "", "  ")
}

func selectedNodes(cfg config.PanelConfig) ([]config.Node, error) {
	if len(cfg.Selection.ActiveNodeIDs) == 0 {
		return nil, fmt.Errorf("no active nodes selected")
	}
	byID := map[string]config.Node{}
	for _, n := range cfg.Nodes {
		byID[n.ID] = n
	}

	order := cfg.Selection.FallbackOrder
	if len(order) == 0 {
		order = cfg.Selection.ActiveNodeIDs
	}

	var nodes []config.Node
	seen := map[string]struct{}{}
	for _, id := range order {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		active := false
		for _, aid := range cfg.Selection.ActiveNodeIDs {
			if aid == id {
				active = true
				break
			}
		}
		if !active {
			continue
		}
		n, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("node not found: %s", id)
		}
		nodes = append(nodes, n)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no active nodes selected")
	}
	return nodes, nil
}

func OutboundTag(nodeID string) string {
	if len(nodeID) > 8 {
		return "proxy-" + nodeID[:8]
	}
	return "proxy-" + nodeID
}

func buildLog(cfg config.PanelConfig) map[string]any {
	logCfg := map[string]any{"loglevel": "warning"}
	if cfg.Logs.XrayAccess != "" {
		logCfg["access"] = cfg.Logs.XrayAccess
	}
	if cfg.Logs.XrayError != "" {
		logCfg["error"] = cfg.Logs.XrayError
	}
	return logCfg
}

func baseInbounds(cfg config.PanelConfig) []map[string]any {
	cfg.Normalize()
	tcpPort := cfg.Iptables.TCPPort
	udpPort := cfg.Iptables.UDPPort
	socksPort := cfg.Iptables.SOCKSPort
	apiPort := cfg.Iptables.APIPort
	return []map[string]any{
		{
			"tag": "redirect-in", "port": tcpPort, "protocol": "dokodemo-door", "listen": "0.0.0.0",
			"settings": map[string]any{"network": "tcp", "followRedirect": true},
			"sniffing": map[string]any{"enabled": true, "destOverride": []string{"http", "tls"}, "routeOnly": true},
		},
		{
			"tag": "tproxy-in", "port": udpPort, "protocol": "dokodemo-door", "listen": "0.0.0.0",
			"settings":       map[string]any{"network": "udp", "followRedirect": true},
			"sniffing":       map[string]any{"enabled": true, "destOverride": []string{"http", "tls", "quic"}, "routeOnly": true},
			"streamSettings": map[string]any{"sockopt": map[string]any{"tproxy": "tproxy"}},
		},
		{
			"tag": "socks-in", "port": socksPort, "protocol": "socks", "listen": "127.0.0.1",
			"settings": map[string]any{"udp": true},
		},
		{
			"tag": "api", "listen": "127.0.0.1", "port": apiPort, "protocol": "dokodemo-door",
			"settings": map[string]any{"address": "127.0.0.1"},
		},
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
