package service

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/taiiok/xiaomi-vless/internal/config"
	"github.com/taiiok/xiaomi-vless/internal/network"
	"github.com/taiiok/xiaomi-vless/internal/xray"
)

type StatusService struct {
	store    *config.Store
	failOpen *FailOpenService
}

func NewStatusService(store *config.Store, failOpen *FailOpenService) *StatusService {
	return &StatusService{store: store, failOpen: failOpen}
}

type StatusResponse struct {
	State           string            `json:"state"`
	XrayRunning     bool              `json:"xray_running"`
	VPNConnected    bool              `json:"vpn_connected"`
	ExitIP          string            `json:"exit_ip,omitempty"`
	ProbeLatencyMs  int               `json:"probe_latency_ms,omitempty"`
	ActiveNodes     []string          `json:"active_nodes"`
	SelectionMode   string            `json:"selection_mode"`
	ActiveOutbound  string            `json:"active_outbound,omitempty"`
	IptablesTCP     IptablesChain     `json:"iptables_tcp"`
	IptablesUDP     IptablesChain     `json:"iptables_udp"`
	Observatory     ObservatoryStatus `json:"observatory"`
	ObservatoryLive bool              `json:"observatory_live"`
	WatchdogAlert   string                     `json:"watchdog_alert,omitempty"`
	WatchdogActive  bool                       `json:"watchdog_active"`
	FailOpenActive  bool                       `json:"fail_open_active"`
	GuestNetwork    network.GuestNetworkStatus `json:"guest_network"`
	CheckedAt       time.Time                  `json:"checked_at"`
	Message         string            `json:"message,omitempty"`
}

type IptablesChain struct {
	Name      string `json:"name"`
	Packets   int64  `json:"packets"`
	Bytes     int64  `json:"bytes"`
	Available bool   `json:"available"`
}

func (s *StatusService) GetStatus(ctx context.Context) StatusResponse {
	cfg := s.store.Get()

	resp := StatusResponse{
		XrayRunning:   IsXrayRunning(),
		ActiveNodes:   cfg.Selection.ActiveNodeIDs,
		SelectionMode: cfg.Selection.Mode,
		CheckedAt:     time.Now(),
		WatchdogAlert: cfg.Watchdog.LastAlert,
		IptablesTCP:   parseIptablesChain("nat", "XRAY_GUEST_TCP"),
		IptablesUDP:   parseIptablesChain("mangle", "XRAY_GUEST_UDP"),
		GuestNetwork:  network.DetectGuest(guestSubnetFromConfig(cfg)),
	}

	start := time.Now()
	exitIP, probeErr := fetchExitIP(ctx, cfg)
	probeLatency := int(time.Since(start).Milliseconds())
	vpnConnected := probeErr == nil

	obs := s.GetObservatory(ctx)
	if vpnConnected && !obs.Live {
		obs = applySOCKSProbeFallback(obs, cfg, probeLatency)
	}
	if obs.ActiveOutbound == "" && vpnConnected {
		obs.ActiveOutbound = primaryProxyTag(cfg)
	}

	resp.Observatory = obs
	resp.ObservatoryLive = obs.Live
	resp.ActiveOutbound = obs.ActiveOutbound

	if vpnConnected {
		resp.VPNConnected = true
		resp.ExitIP = exitIP
		resp.ProbeLatencyMs = probeLatency
	}

	resp.WatchdogActive, _ = DetectWatchdogOutage(resp.XrayRunning, resp.VPNConnected, resp.Observatory)
	if s.failOpen != nil {
		resp.FailOpenActive = s.failOpen.IsActive()
	}

	switch {
	case !resp.XrayRunning:
		resp.State = "stopped"
		resp.Message = "xray process not running"
	case resp.VPNConnected:
		resp.State = "running"
	case resp.XrayRunning:
		resp.State = "degraded"
		resp.Message = "xray running but VPN probe failed"
	default:
		resp.State = "stopped"
	}
	return resp
}

func primaryProxyTag(cfg config.PanelConfig) string {
	for _, id := range cfg.Selection.ActiveNodeIDs {
		for _, node := range cfg.Nodes {
			if node.ID == id {
				return xray.OutboundTag(node.ID)
			}
		}
	}
	return ""
}

func applySOCKSProbeFallback(obs ObservatoryStatus, cfg config.PanelConfig, latencyMs int) ObservatoryStatus {
	if len(obs.Nodes) == 0 {
		return obs
	}
	tag := primaryProxyTag(cfg)
	for i := range obs.Nodes {
		n := &obs.Nodes[i]
		if n.Source == "xray" && n.Health != "" && n.Health != "unknown" {
			continue
		}
		n.Health = "ok"
		n.Alive = true
		if latencyMs > 0 {
			n.LatencyMs = latencyMs
		}
		n.Source = "socks-probe"
		n.LastError = ""
	}
	if obs.ActiveOutbound == "" && tag != "" {
		obs.ActiveOutbound = tag
	}
	return obs
}

func fetchExitIP(ctx context.Context, cfg config.PanelConfig) (string, error) {
	proxy := fmt.Sprintf("socks5h://127.0.0.1:%d", cfg.Iptables.SOCKSPort)
	dialer := contextAwareCurl(ctx, proxy, "https://api.ipify.org")
	out, err := dialer.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func contextAwareCurl(ctx context.Context, proxy, url string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "curl", "-4", "-sS", "--connect-timeout", "5", "-x", proxy, url)
	return cmd
}

var iptablesLine = regexp.MustCompile(`^\s*(\d+)\s+(\d+)`)

func parseIptablesChain(table, chain string) IptablesChain {
	result := IptablesChain{Name: chain}
	out, err := exec.Command("iptables", "-t", table, "-L", chain, "-v", "-n").Output()
	if err != nil {
		return result
	}
	result.Available = true
	lines := strings.Split(string(out), "\n")
	var totalPkts, totalBytes int64
	for _, line := range lines {
		m := iptablesLine.FindStringSubmatch(line)
		if len(m) != 3 {
			continue
		}
		pkts, _ := strconv.ParseInt(m[1], 10, 64)
		bytes, _ := strconv.ParseInt(m[2], 10, 64)
		if strings.Contains(line, "REDIRECT") || strings.Contains(line, "TPROXY") {
			totalPkts += pkts
			totalBytes += bytes
		}
	}
	result.Packets = totalPkts
	result.Bytes = totalBytes
	return result
}

type ObservatoryStatus struct {
	Live           bool         `json:"live"`
	ActiveOutbound string       `json:"active_outbound,omitempty"`
	Nodes          []NodeHealth `json:"nodes"`
	Message        string       `json:"message,omitempty"`
}

type NodeHealth struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Tag       string `json:"tag"`
	Health    string `json:"health"`
	LatencyMs int    `json:"latency_ms,omitempty"`
	Alive     bool   `json:"alive"`
	LastError string `json:"last_error,omitempty"`
	Source    string `json:"source"`
}

func (s *StatusService) apiClient(cfg config.PanelConfig) *xray.APIClient {
	addr := cfg.Network.XrayAPIAddr
	if addr == "" {
		addr = xray.DefaultAPIAddr
	}
	return xray.NewAPIClient(addr)
}

func (s *StatusService) GetObservatory(ctx context.Context) ObservatoryStatus {
	cfg := s.store.Get()
	out := ObservatoryStatus{Nodes: []NodeHealth{}}

	live, liveErr := s.apiClient(cfg).GetOutboundStatuses(ctx)
	liveByTag := map[string]xray.LiveOutboundStatus{}
	for _, st := range live {
		liveByTag[st.OutboundTag] = st
	}
	out.Live = liveErr == nil && len(live) > 0
	if liveErr != nil {
		out.Message = liveErr.Error()
	} else if len(live) > 0 {
		out.ActiveOutbound = xray.PickBestAlive(live)
	}

	active := map[string]struct{}{}
	for _, id := range cfg.Selection.ActiveNodeIDs {
		active[id] = struct{}{}
	}

	for _, node := range cfg.Nodes {
		if _, ok := active[node.ID]; !ok {
			continue
		}
		tag := xray.OutboundTag(node.ID)
		health := NodeHealth{
			ID:     node.ID,
			Name:   node.Name,
			Tag:    tag,
			Source: "cache",
		}

		if st, ok := liveByTag[tag]; ok && st.Available {
			health.Source = "xray"
			health.Alive = st.Alive
			health.LatencyMs = int(st.DelayMs)
			health.LastError = st.LastError
			if st.Alive {
				health.Health = "alive"
			} else {
				health.Health = "dead"
			}
		} else {
			health.Health = node.LastHealth
			if health.Health == "" {
				health.Health = "unknown"
			}
			health.LatencyMs = node.LastLatencyMs
			health.Alive = health.Health == "ok" || health.Health == "alive"
			health.Source = "cache"
		}

		out.Nodes = append(out.Nodes, health)
	}

	if out.ActiveOutbound == "" && len(out.Nodes) == 1 && out.Nodes[0].Alive {
		out.ActiveOutbound = out.Nodes[0].Tag
	}

	return out
}

func probeNodeIDFilter(nodeIDs []string) map[string]struct{} {
	ids := map[string]struct{}{}
	for _, id := range nodeIDs {
		ids[id] = struct{}{}
	}
	return ids
}

func (s *StatusService) ProbeNodes(ctx context.Context, nodeIDs []string) error {
	cfg := s.store.Get()
	return s.store.Update(func(c *config.PanelConfig) error {
		ids := probeNodeIDFilter(nodeIDs)
		for i, node := range c.Nodes {
			if len(ids) > 0 {
				if _, ok := ids[node.ID]; !ok {
					continue
				}
			}
			tcpMs, tcpHealth := probeNodeTCP(ctx, node)
			c.Nodes[i].LastLatencyMs = tcpMs
			c.Nodes[i].LastHealth = tcpHealth

			xrayMs, xrayHealth := s.probeNodeXray(ctx, cfg, node)
			c.Nodes[i].LastXrayLatencyMs = xrayMs
			c.Nodes[i].LastXrayHealth = xrayHealth
		}
		return nil
	})
}

func probeNodeTCP(ctx context.Context, node config.Node) (latencyMs int, health string) {
	if node.Address == "" || node.Port <= 0 {
		return 0, "dead"
	}
	ms, err := TCPLatencyMs(ctx, node.Address, node.Port)
	if err != nil {
		return 0, "dead"
	}
	return ms, "ok"
}

type PingNodeResult struct {
	LatencyMs      int    `json:"latency_ms"`
	Health         string `json:"health"`
	XrayLatencyMs  int    `json:"xray_latency_ms,omitempty"`
	XrayHealth     string `json:"xray_health"`
}

func (s *StatusService) PingNode(ctx context.Context, nodeID string) (PingNodeResult, error) {
	cfg := s.store.Get()
	var node config.Node
	found := false
	for _, n := range cfg.Nodes {
		if n.ID == nodeID {
			node = n
			found = true
			break
		}
	}
	if !found {
		return PingNodeResult{}, fmt.Errorf("node not found")
	}
	tcpMs, tcpHealth := probeNodeTCP(ctx, node)
	xrayMs, xrayHealth := s.probeNodeXray(ctx, cfg, node)
	result := PingNodeResult{
		LatencyMs:     tcpMs,
		Health:        tcpHealth,
		XrayLatencyMs: xrayMs,
		XrayHealth:    xrayHealth,
	}
	_ = s.store.Update(func(c *config.PanelConfig) error {
		for i, n := range c.Nodes {
			if n.ID == nodeID {
				c.Nodes[i].LastLatencyMs = result.LatencyMs
				c.Nodes[i].LastHealth = result.Health
				c.Nodes[i].LastXrayLatencyMs = result.XrayLatencyMs
				c.Nodes[i].LastXrayHealth = result.XrayHealth
				break
			}
		}
		return nil
	})
	return result, nil
}
