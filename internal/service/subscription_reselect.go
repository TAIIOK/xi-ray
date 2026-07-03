package service

import (
	"context"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

func nodeHashSet(nodes []config.Node) map[string]struct{} {
	out := make(map[string]struct{}, len(nodes))
	for _, n := range nodes {
		if n.Hash != "" {
			out[n.Hash] = struct{}{}
		}
	}
	return out
}

func hashSetsEqual(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

func sameStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func filterValidActiveIDs(cfg *config.PanelConfig) []string {
	byID := make(map[string]struct{}, len(cfg.Nodes))
	for _, n := range cfg.Nodes {
		byID[n.ID] = struct{}{}
	}
	out := make([]string, 0, len(cfg.Selection.ActiveNodeIDs))
	for _, id := range cfg.Selection.ActiveNodeIDs {
		if _, ok := byID[id]; ok {
			out = append(out, id)
		}
	}
	return out
}

func preferVlessNodes(nodes []config.Node) []config.Node {
	out := vlessNodesFromStore(nodes)
	if len(out) > 0 {
		return out
	}
	return nodes
}

func pickNodeIDsByStrategy(ctx context.Context, nodes []config.Node, strategy string) []string {
	candidates := preferVlessNodes(nodes)
	if len(candidates) == 0 {
		return nil
	}
	switch strings.ToLower(strategy) {
	case "keep":
		return nil
	case "best_ping":
		if id, ms := pickLowestTCPLatency(ctx, candidates); id != "" {
			for i := range candidates {
				if candidates[i].ID == id {
					candidates[i].LastLatencyMs = ms
					candidates[i].LastHealth = "ok"
				}
			}
			return []string{id}
		}
		fallthrough
	default:
		return []string{candidates[0].ID}
	}
}

func pickLowestTCPLatency(ctx context.Context, nodes []config.Node) (string, int) {
	var bestID string
	bestMs := -1
	for _, n := range nodes {
		if n.Address == "" || n.Port <= 0 {
			continue
		}
		ms, err := tcpLatencyMs(ctx, n.Address, n.Port)
		if err != nil {
			continue
		}
		if bestMs < 0 || ms < bestMs {
			bestMs = ms
			bestID = n.ID
		}
	}
	return bestID, bestMs
}

func tcpLatencyMs(ctx context.Context, host string, port int) (int, error) {
	d := net.Dialer{Timeout: 5 * time.Second}
	start := time.Now()
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return 0, err
	}
	_ = conn.Close()
	return int(time.Since(start).Milliseconds()), nil
}

// reselectAfterSubscriptionUpdate picks a new active node when the subscription list changed
// or the current selection is no longer valid. Returns true if selection changed.
func reselectAfterSubscriptionUpdate(cfg *config.PanelConfig, subID string, oldHashes map[string]struct{}, strategy string) bool {
	subNodes := nodesBySubscription(cfg.Nodes, subID)
	newHashes := nodeHashSet(subNodes)
	listChanged := !hashSetsEqual(oldHashes, newHashes)

	activeValid := filterValidActiveIDs(cfg)
	activeStale := len(activeValid) != len(cfg.Selection.ActiveNodeIDs)

	pickStrategy := strategy
	shouldReselect := false
	switch strings.ToLower(strategy) {
	case "keep":
		if len(activeValid) == 0 {
			shouldReselect = true
			pickStrategy = "first"
		} else {
			cfg.Selection.ActiveNodeIDs = activeValid
			if len(cfg.Selection.FallbackOrder) == 0 {
				cfg.Selection.FallbackOrder = activeValid
			}
			return false
		}
	default:
		if listChanged || activeStale {
			shouldReselect = true
		}
	}

	if !shouldReselect {
		cfg.Selection.ActiveNodeIDs = activeValid
		return false
	}

	oldActive := append([]string(nil), cfg.Selection.ActiveNodeIDs...)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	picked := pickNodeIDsByStrategy(ctx, subNodes, pickStrategy)
	if len(picked) == 0 {
		sanitizeSelection(cfg)
		return !sameStringSlice(oldActive, cfg.Selection.ActiveNodeIDs)
	}

	if cfg.Selection.Mode == "single" && len(picked) > 1 {
		picked = picked[:1]
	}
	cfg.Selection.ActiveNodeIDs = picked
	if len(cfg.Selection.FallbackOrder) == 0 || listChanged || activeStale {
		cfg.Selection.FallbackOrder = append([]string(nil), picked...)
	}

	for i, n := range cfg.Nodes {
		for _, p := range subNodes {
			if n.ID == p.ID && p.LastLatencyMs > 0 {
				cfg.Nodes[i].LastLatencyMs = p.LastLatencyMs
				cfg.Nodes[i].LastHealth = p.LastHealth
			}
		}
	}

	return !sameStringSlice(oldActive, cfg.Selection.ActiveNodeIDs)
}
