package subscription

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/taiiok/xiaomi-vless/internal/config"
)

func ParseSubscriptionBody(body []byte) ([]config.Node, error) {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return nil, fmt.Errorf("empty subscription body")
	}

	decoded := text
	if !strings.Contains(text, "://") {
		if b, err := base64.StdEncoding.DecodeString(padBase64(text)); err == nil {
			decoded = string(b)
		} else if b, err := base64.RawStdEncoding.DecodeString(text); err == nil {
			decoded = string(b)
		}
	}

	lines := splitLines(decoded)
	var nodes []config.Node
	var errs []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		node, err := ParseLink(line)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("no valid nodes: %s", strings.Join(errs, "; "))
	}
	return nodes, nil
}

func ParseLink(raw string) (config.Node, error) {
	raw = strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(raw, "vless://"):
		return parseVLESS(raw)
	case strings.HasPrefix(raw, "vmess://"):
		return parseVMess(raw)
	case strings.HasPrefix(raw, "trojan://"):
		return parseTrojan(raw)
	case strings.HasPrefix(raw, "ss://"):
		return parseShadowsocks(raw)
	default:
		return config.Node{}, fmt.Errorf("unsupported link scheme: %s", raw[:min(12, len(raw))])
	}
}

func parseVLESS(raw string) (config.Node, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return config.Node{}, fmt.Errorf("parse vless url: %w", err)
	}
	if u.Scheme != "vless" {
		return config.Node{}, fmt.Errorf("not a vless link")
	}

	uuidStr := u.User.Username()
	if uuidStr == "" {
		return config.Node{}, fmt.Errorf("missing uuid")
	}

	port := 443
	if u.Port() != "" {
		p, err := strconv.Atoi(u.Port())
		if err != nil {
			return config.Node{}, fmt.Errorf("invalid port: %w", err)
		}
		port = p
	}

	q := u.Query()
	security := strings.ToLower(q.Get("security"))
	if security == "" {
		security = "none"
	}
	network := strings.ToLower(q.Get("type"))
	if network == "" {
		network = "tcp"
	}

	name := u.Fragment
	if name == "" {
		name = fmt.Sprintf("%s:%d", u.Hostname(), port)
	}

	node := config.Node{
		ID:          uuid.NewString(),
		Name:        name,
		Protocol:    "vless",
		Address:     u.Hostname(),
		Port:        port,
		UUID:        uuidStr,
		Security:    security,
		Network:     network,
		Flow:        q.Get("flow"),
		SNI:         firstNonEmpty(q.Get("sni"), q.Get("host")),
		Fingerprint: q.Get("fp"),
		PublicKey:   q.Get("pbk"),
		ShortID:     q.Get("sid"),
		SpiderX:     q.Get("spx"),
		Path:        firstNonEmpty(q.Get("path"), q.Get("serviceName")),
		Host:        q.Get("host"),
		RawLink:     raw,
		UpdatedAt:   time.Now(),
	}
	applyTransportQuery(&node, q)
	node.Hash = NodeHash(node)
	return node, nil
}

func applyTransportQuery(node *config.Node, q url.Values) {
	network := strings.ToLower(node.Network)
	switch network {
	case "xhttp", "splithttp":
		node.Network = "xhttp"
		node.XHTTPMode = q.Get("mode")
		if extra := parseXHTTPExtra(q.Get("extra")); len(extra) > 0 {
			node.XHTTPExtra = extra
		}
		mergeXHTTPQueryParams(node, q)
	case "grpc":
		if node.Path == "" {
			node.Path = q.Get("serviceName")
		}
		if node.Host == "" {
			node.Host = q.Get("authority")
		}
	}
}

func parseXHTTPExtra(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if decoded, err := url.QueryUnescape(raw); err == nil && decoded != "" {
		raw = decoded
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		if b, err := base64.StdEncoding.DecodeString(padBase64(raw)); err == nil {
			_ = json.Unmarshal(b, &out)
		} else if b, err := base64.RawStdEncoding.DecodeString(raw); err == nil {
			_ = json.Unmarshal(b, &out)
		}
	}
	return out
}

func mergeXHTTPQueryParams(node *config.Node, q url.Values) {
	direct := []string{
		"xPaddingBytes", "scMaxEachPostBytes", "scMinPostsIntervalMs",
		"scMaxBufferedPosts", "scStreamUpServerSecs", "noGRPCHeader",
	}
	for _, key := range direct {
		if v := strings.TrimSpace(q.Get(key)); v != "" {
			if node.XHTTPExtra == nil {
				node.XHTTPExtra = map[string]any{}
			}
			node.XHTTPExtra[key] = v
		}
	}
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.Split(s, "\n")
}

func padBase64(s string) string {
	switch len(s) % 4 {
	case 2:
		return s + "=="
	case 3:
		return s + "="
	default:
		return s
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
