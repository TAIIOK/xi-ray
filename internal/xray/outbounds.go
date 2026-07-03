package xray

import (
	"fmt"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

func nodeToOutbound(tag string, node config.Node) (map[string]any, error) {
	switch node.Protocol {
	case "vless":
		return vlessOutbound(tag, node)
	case "vmess":
		return vmessOutbound(tag, node)
	case "trojan":
		return trojanOutbound(tag, node)
	case "shadowsocks":
		return shadowsocksOutbound(tag, node)
	default:
		return nil, fmt.Errorf("unsupported protocol %s", node.Protocol)
	}
}

func vlessOutbound(tag string, node config.Node) (map[string]any, error) {
	user := map[string]any{
		"id":         node.UUID,
		"encryption": "none",
	}
	if node.Flow != "" && normalizeNetwork(node.Network) == "tcp" {
		user["flow"] = node.Flow
	}

	outbound := map[string]any{
		"tag":      tag,
		"protocol": "vless",
		"settings": map[string]any{
			"vnext": []any{
				map[string]any{
					"address": node.Address,
					"port":    node.Port,
					"users":   []any{user},
				},
			},
		},
	}
	outbound["streamSettings"] = buildStreamSettings(node)
	return outbound, nil
}

func vmessOutbound(tag string, node config.Node) (map[string]any, error) {
	user := map[string]any{
		"id":       node.UUID,
		"alterId":  node.AlterID,
		"security": "auto",
	}
	outbound := map[string]any{
		"tag":      tag,
		"protocol": "vmess",
		"settings": map[string]any{
			"vnext": []any{
				map[string]any{
					"address": node.Address,
					"port":    node.Port,
					"users":   []any{user},
				},
			},
		},
	}
	outbound["streamSettings"] = buildStreamSettings(node)
	return outbound, nil
}

func trojanOutbound(tag string, node config.Node) (map[string]any, error) {
	outbound := map[string]any{
		"tag":      tag,
		"protocol": "trojan",
		"settings": map[string]any{
			"servers": []any{
				map[string]any{
					"address":  node.Address,
					"port":     node.Port,
					"password": node.Password,
				},
			},
		},
	}
	outbound["streamSettings"] = buildStreamSettings(node)
	return outbound, nil
}

func shadowsocksOutbound(tag string, node config.Node) (map[string]any, error) {
	method := node.Method
	if method == "" {
		method = "aes-256-gcm"
	}
	outbound := map[string]any{
		"tag":      tag,
		"protocol": "shadowsocks",
		"settings": map[string]any{
			"servers": []any{
				map[string]any{
					"address":  node.Address,
					"port":     node.Port,
					"method":   method,
					"password": node.Password,
				},
			},
		},
	}
	if node.Network != "" && node.Network != "tcp" {
		outbound["streamSettings"] = buildStreamSettings(node)
	}
	return outbound, nil
}
