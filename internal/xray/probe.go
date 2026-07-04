package xray

import (
	"encoding/json"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

const probeOutboundTag = "proxy-probe"

// GenerateProbeConfig builds a minimal Xray config that routes all SOCKS traffic through one node.
func GenerateProbeConfig(node config.Node, socksPort int) ([]byte, error) {
	ob, err := nodeToOutbound(probeOutboundTag, node)
	if err != nil {
		return nil, err
	}
	doc := map[string]any{
		"log": map[string]any{"loglevel": "error"},
		"inbounds": []any{
			map[string]any{
				"tag": "socks-in", "port": socksPort, "protocol": "socks", "listen": "127.0.0.1",
				"settings": map[string]any{"udp": true},
			},
		},
		"outbounds": []any{
			ob,
			map[string]any{"tag": "direct", "protocol": "freedom"},
			map[string]any{"tag": "block", "protocol": "blackhole"},
		},
		"routing": map[string]any{
			"domainStrategy": "AsIs",
			"rules": []any{
				map[string]any{
					"type": "field", "inboundTag": []string{"socks-in"}, "outboundTag": probeOutboundTag,
				},
			},
		},
	}
	return json.MarshalIndent(doc, "", "  ")
}

func ProbeOutboundTag() string { return probeOutboundTag }
