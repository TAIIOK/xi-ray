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

type vmessPayload struct {
	V    string `json:"v"`
	PS   string `json:"ps"`
	Add  string `json:"add"`
	Port any    `json:"port"`
	ID   string `json:"id"`
	Aid  any    `json:"aid"`
	Net  string `json:"net"`
	Type string `json:"type"`
	Host string `json:"host"`
	Path string `json:"path"`
	TLS  string `json:"tls"`
	SNI  string `json:"sni"`
	FP   string `json:"fp"`
	Scy  string `json:"scy"`
}

func parseVMess(raw string) (config.Node, error) {
	payload := strings.TrimPrefix(raw, "vmess://")
	b, err := base64.StdEncoding.DecodeString(padBase64(payload))
	if err != nil {
		if b, err = base64.RawStdEncoding.DecodeString(payload); err != nil {
			return config.Node{}, fmt.Errorf("decode vmess payload: %w", err)
		}
	}

	var m vmessPayload
	if err := json.Unmarshal(b, &m); err != nil {
		return config.Node{}, fmt.Errorf("parse vmess json: %w", err)
	}
	if m.Add == "" || m.ID == "" {
		return config.Node{}, fmt.Errorf("vmess missing address or uuid")
	}

	port := parsePortAny(m.Port, 443)
	network := strings.ToLower(firstNonEmpty(m.Net, "tcp"))
	security := "none"
	if strings.EqualFold(m.TLS, "tls") {
		security = "tls"
	}

	name := m.PS
	if name == "" {
		name = fmt.Sprintf("%s:%d", m.Add, port)
	}

	node := config.Node{
		ID:          uuid.NewString(),
		Name:        name,
		Protocol:    "vmess",
		Address:     m.Add,
		Port:        port,
		UUID:        m.ID,
		AlterID:     parseIntAny(m.Aid),
		Security:    security,
		Network:     network,
		SNI:         firstNonEmpty(m.SNI, m.Host),
		Fingerprint: m.FP,
		Path:        m.Path,
		Host:        m.Host,
		RawLink:     raw,
		UpdatedAt:   time.Now(),
	}
	if strings.EqualFold(network, "xhttp") || strings.EqualFold(network, "splithttp") {
		node.Network = "xhttp"
		node.XHTTPMode = firstNonEmpty(m.Type, "auto")
	}
	node.Hash = NodeHash(node)
	return node, nil
}

func parseTrojan(raw string) (config.Node, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return config.Node{}, fmt.Errorf("parse trojan url: %w", err)
	}
	if u.Scheme != "trojan" {
		return config.Node{}, fmt.Errorf("not a trojan link")
	}

	password := u.User.Username()
	if password == "" {
		return config.Node{}, fmt.Errorf("missing trojan password")
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
	security := "tls"
	if q.Get("allowInsecure") == "1" || q.Get("allowInsecure") == "true" {
		security = "none"
	}

	name := u.Fragment
	if name == "" {
		name = fmt.Sprintf("%s:%d", u.Hostname(), port)
	}

	node := config.Node{
		ID:          uuid.NewString(),
		Name:        name,
		Protocol:    "trojan",
		Address:     u.Hostname(),
		Port:        port,
		Password:    password,
		Security:    security,
		Network:     "tcp",
		SNI:         firstNonEmpty(q.Get("sni"), q.Get("peer"), u.Hostname()),
		Fingerprint: q.Get("fp"),
		RawLink:     raw,
		UpdatedAt:   time.Now(),
	}
	node.Hash = NodeHash(node)
	return node, nil
}

func parseShadowsocks(raw string) (config.Node, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return config.Node{}, fmt.Errorf("parse ss url: %w", err)
	}
	if u.Scheme != "ss" {
		return config.Node{}, fmt.Errorf("not a shadowsocks link")
	}

	method := ""
	password := ""
	host := u.Hostname()
	port := 8388

	if u.User != nil {
		user := u.User.Username()
		password, _ = u.User.Password()
		if password == "" && strings.Contains(user, ":") {
			method = user
		} else if password != "" {
			method = user
		} else {
			decoded, err := decodeSSUserinfo(user)
			if err != nil {
				return config.Node{}, err
			}
			creds := strings.SplitN(decoded, ":", 2)
			if len(creds) != 2 {
				return config.Node{}, fmt.Errorf("invalid ss credentials")
			}
			method, password = creds[0], creds[1]
		}
	} else if u.Host != "" {
		// ss://base64(method:password@host:port)
		decoded, err := decodeSSUserinfo(u.Host)
		if err != nil {
			return config.Node{}, err
		}
		parts := strings.SplitN(decoded, "@", 2)
		if len(parts) != 2 {
			return config.Node{}, fmt.Errorf("invalid ss payload")
		}
		creds := strings.SplitN(parts[0], ":", 2)
		if len(creds) != 2 {
			return config.Node{}, fmt.Errorf("invalid ss credentials")
		}
		method, password = creds[0], creds[1]
		hostPort := parts[1]
		if hp, err := url.Parse("ss://" + hostPort); err == nil {
			host = hp.Hostname()
			if hp.Port() != "" {
				port, _ = strconv.Atoi(hp.Port())
			}
		} else {
			hostParts := strings.Split(hostPort, ":")
			host = hostParts[0]
			if len(hostParts) > 1 {
				port, _ = strconv.Atoi(hostParts[1])
			}
		}
	}

	if u.Port() != "" {
		p, err := strconv.Atoi(u.Port())
		if err != nil {
			return config.Node{}, fmt.Errorf("invalid port: %w", err)
		}
		port = p
	}

	if method == "" || password == "" || host == "" {
		return config.Node{}, fmt.Errorf("ss link missing method, password or host")
	}

	name := u.Fragment
	if name == "" {
		name = fmt.Sprintf("%s:%d", host, port)
	}

	node := config.Node{
		ID:        uuid.NewString(),
		Name:      name,
		Protocol:  "shadowsocks",
		Address:   host,
		Port:      port,
		Method:    method,
		Password:  password,
		Security:  "none",
		Network:   "tcp",
		RawLink:   raw,
		UpdatedAt: time.Now(),
	}
	node.Hash = NodeHash(node)
	return node, nil
}

func decodeSSUserinfo(host string) (string, error) {
	if b, err := base64.RawURLEncoding.DecodeString(host); err == nil {
		return string(b), nil
	}
	if b, err := base64.URLEncoding.DecodeString(host); err == nil {
		return string(b), nil
	}
	if b, err := base64.StdEncoding.DecodeString(padBase64(host)); err == nil {
		return string(b), nil
	}
	return "", fmt.Errorf("decode ss userinfo")
}

func parsePortAny(v any, fallback int) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case string:
		if t == "" {
			return fallback
		}
		p, err := strconv.Atoi(t)
		if err != nil {
			return fallback
		}
		return p
	default:
		return fallback
	}
}

func parseIntAny(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case string:
		n, _ := strconv.Atoi(t)
		return n
	default:
		return 0
	}
}
