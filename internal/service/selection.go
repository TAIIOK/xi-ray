package service

import (
	"strings"

	"github.com/taiiok/xiaomi-vless/internal/config"
	"github.com/taiiok/xiaomi-vless/internal/subscription"
)

func nodesBySubscription(nodes []config.Node, subID string) []config.Node {
	out := make([]config.Node, 0)
	for _, n := range nodes {
		if n.SubscriptionID == subID {
			out = append(out, n)
		}
	}
	return out
}

func nodeByHash(nodes []config.Node, hash string) (config.Node, bool) {
	for _, n := range nodes {
		if n.Hash == hash {
			return n, true
		}
	}
	return config.Node{}, false
}

// sanitizeSelection drops stale node IDs and picks a default VLESS node when needed.
func sanitizeSelection(cfg *config.PanelConfig) {
	byID := make(map[string]config.Node, len(cfg.Nodes))
	for _, n := range cfg.Nodes {
		byID[n.ID] = n
	}

	filter := func(ids []string) []string {
		out := make([]string, 0, len(ids))
		for _, id := range ids {
			if _, ok := byID[id]; ok {
				out = append(out, id)
			}
		}
		return out
	}

	active := filter(cfg.Selection.ActiveNodeIDs)
	fallback := filter(cfg.Selection.FallbackOrder)

	if len(active) == 0 {
		for _, n := range cfg.Nodes {
			if strings.EqualFold(n.Protocol, "vless") {
				active = []string{n.ID}
				break
			}
		}
	}
	if len(active) == 0 && len(cfg.Nodes) > 0 {
		active = []string{cfg.Nodes[0].ID}
	}
	if len(fallback) == 0 {
		fallback = active
	} else {
		fallback = filter(fallback)
		if len(fallback) == 0 {
			fallback = active
		}
	}

	cfg.Selection.ActiveNodeIDs = active
	cfg.Selection.FallbackOrder = fallback
}

func firstVlessNodeID(nodes []config.Node) string {
	for _, n := range nodes {
		if strings.EqualFold(n.Protocol, "vless") {
			return n.ID
		}
	}
	return ""
}

func vlessNodesFromStore(nodes []config.Node) []config.Node {
	return subscription.FilterByProtocol(nodes, "vless")
}
