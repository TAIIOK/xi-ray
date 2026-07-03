package subscription

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/taiiok/xiaomi-vless/internal/config"
)

func NodeHash(node config.Node) string {
	identity := node.UUID
	switch strings.ToLower(node.Protocol) {
	case "trojan", "shadowsocks":
		identity = node.Password
	case "vmess":
		if identity == "" {
			identity = node.UUID
		}
	}
	raw := fmt.Sprintf("%s:%s:%d:%s:%s:%s:%s:%s:%s",
		strings.ToLower(node.Protocol),
		strings.ToLower(node.Address),
		node.Port,
		identity,
		node.Security,
		node.Method,
		strings.ToLower(node.Network),
		node.Path,
		node.XHTTPMode,
	)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:8])
}

// NodeHashLegacy keeps compatibility with older hash format for vless-only nodes.
func NodeHashLegacy(address string, port int, uuidStr, security string) string {
	return NodeHash(config.Node{
		Protocol: "vless",
		Address:  address,
		Port:     port,
		UUID:     uuidStr,
		Security: security,
	})
}

func MergeNodes(existing []config.Node, incoming []config.Node) []config.Node {
	byHash := make(map[string]config.Node, len(existing))
	order := make([]string, 0, len(existing))
	for _, n := range existing {
		if n.Hash == "" {
			n.Hash = NodeHash(n)
		}
		byHash[n.Hash] = n
		order = append(order, n.Hash)
	}
	for _, n := range incoming {
		if n.Hash == "" {
			n.Hash = NodeHash(n)
		}
		if prev, ok := byHash[n.Hash]; ok {
			n.ID = prev.ID
			n.LastLatencyMs = prev.LastLatencyMs
			n.LastHealth = prev.LastHealth
			byHash[n.Hash] = n
		} else {
			if n.ID == "" {
				n.ID = uuid.NewString()
			}
			byHash[n.Hash] = n
			order = append(order, n.Hash)
		}
	}
	out := make([]config.Node, 0, len(order))
	seen := map[string]struct{}{}
	for _, h := range order {
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		out = append(out, byHash[h])
	}
	return out
}

func RemoveNodesBySubscription(nodes []config.Node, subscriptionID string) []config.Node {
	out := make([]config.Node, 0, len(nodes))
	for _, n := range nodes {
		if n.SubscriptionID == subscriptionID {
			continue
		}
		out = append(out, n)
	}
	return out
}

func NewManualNode(link string, parsed config.Node) config.Node {
	parsed.ID = uuid.NewString()
	parsed.Manual = true
	parsed.RawLink = link
	parsed.UpdatedAt = time.Now()
	if parsed.Hash == "" {
		parsed.Hash = NodeHash(parsed)
	}
	return parsed
}
