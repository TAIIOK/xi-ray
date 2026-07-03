package xray

import (
	"strings"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

func normalizeNetwork(network string) string {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "splithttp", "xhttp":
		return "xhttp"
	case "websocket", "ws":
		return "ws"
	case "grpc":
		return "grpc"
	case "httpupgrade", "http-upgrade":
		return "httpupgrade"
	case "h2", "http":
		return "h2"
	default:
		if network == "" {
			return "tcp"
		}
		return strings.ToLower(network)
	}
}

func buildStreamSettings(node config.Node) map[string]any {
	network := normalizeNetwork(node.Network)
	stream := map[string]any{"network": network}

	switch node.Security {
	case "reality":
		stream["security"] = "reality"
		stream["realitySettings"] = map[string]any{
			"serverName":  node.SNI,
			"fingerprint": node.Fingerprint,
			"publicKey":   node.PublicKey,
			"shortId":     node.ShortID,
			"spiderX":     node.SpiderX,
		}
	case "tls":
		stream["security"] = "tls"
		stream["tlsSettings"] = map[string]any{
			"serverName":  node.SNI,
			"fingerprint": node.Fingerprint,
		}
	default:
		if node.Protocol == "trojan" {
			stream["security"] = "tls"
			stream["tlsSettings"] = map[string]any{
				"serverName":  firstNonEmpty(node.SNI, node.Address),
				"fingerprint": node.Fingerprint,
			}
		} else {
			stream["security"] = "none"
		}
	}

	switch network {
	case "ws":
		stream["wsSettings"] = map[string]any{
			"path":    node.Path,
			"headers": map[string]any{"Host": firstNonEmpty(node.Host, node.SNI)},
		}
	case "xhttp":
		stream["xhttpSettings"] = buildXHTTPSettings(node)
	case "grpc":
		grpc := map[string]any{}
		if node.Path != "" {
			grpc["serviceName"] = node.Path
		}
		if h := firstNonEmpty(node.Host, node.SNI); h != "" {
			grpc["authority"] = h
		}
		stream["grpcSettings"] = grpc
	case "httpupgrade":
		stream["httpupgradeSettings"] = map[string]any{
			"path": node.Path,
			"host": firstNonEmpty(node.Host, node.SNI),
		}
	case "h2":
		stream["httpSettings"] = map[string]any{
			"path": node.Path,
			"host": []string{firstNonEmpty(node.Host, node.SNI, node.Address)},
		}
	}
	return stream
}

func buildXHTTPSettings(node config.Node) map[string]any {
	settings := map[string]any{}
	path := node.Path
	if path == "" {
		path = "/"
	}
	settings["path"] = path
	if h := firstNonEmpty(node.Host, node.SNI); h != "" {
		settings["host"] = h
	}
	if node.XHTTPMode != "" {
		settings["mode"] = node.XHTTPMode
	}
	if len(node.XHTTPExtra) > 0 {
		settings["extra"] = node.XHTTPExtra
	}
	return settings
}
