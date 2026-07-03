package subscription

import (
	"strings"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

// IsSubscriptionURL reports whether input looks like an HTTP(S) subscription URL.
func IsSubscriptionURL(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// IsProxyLink reports whether input looks like a single proxy share link.
func IsProxyLink(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	for _, p := range []string{"vless://", "vmess://", "trojan://", "ss://"} {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func FilterByProtocol(nodes []config.Node, protocol string) []config.Node {
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	out := make([]config.Node, 0, len(nodes))
	for _, n := range nodes {
		if strings.EqualFold(n.Protocol, protocol) {
			out = append(out, n)
		}
	}
	return out
}
