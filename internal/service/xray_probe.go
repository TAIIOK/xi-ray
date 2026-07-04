package service

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/taiiok/xiaomi-vless/internal/config"
	"github.com/taiiok/xiaomi-vless/internal/xray"
)

func probeURL(cfg config.PanelConfig) string {
	if cfg.Observatory.ProbeURL != "" {
		return cfg.Observatory.ProbeURL
	}
	return "https://www.google.com/generate_204"
}

func nodeIsActive(cfg config.PanelConfig, nodeID string) bool {
	for _, id := range cfg.Selection.ActiveNodeIDs {
		if id == nodeID {
			return true
		}
	}
	return false
}

func observatoryXrayPing(ctx context.Context, s *StatusService, cfg config.PanelConfig, node config.Node) (int, bool) {
	if !nodeIsActive(cfg, node.ID) || !IsXrayRunning() {
		return 0, false
	}
	tag := xray.OutboundTag(node.ID)
	statuses, err := s.apiClient(cfg).GetOutboundStatuses(ctx)
	if err != nil {
		return 0, false
	}
	for _, st := range statuses {
		if st.OutboundTag != tag || !st.Available {
			continue
		}
		if st.Alive && st.DelayMs > 0 {
			return int(st.DelayMs), true
		}
		return 0, true
	}
	return 0, false
}

func (s *StatusService) probeNodeXray(ctx context.Context, cfg config.PanelConfig, node config.Node) (latencyMs int, health string) {
	if node.Address == "" || node.Port <= 0 {
		return 0, "dead"
	}
	if ms, ok := observatoryXrayPing(ctx, s, cfg, node); ok {
		if ms > 0 {
			return ms, "ok"
		}
		return 0, "dead"
	}
	ms, err := XrayProbeNode(ctx, cfg.Paths.XrayBin, probeURL(cfg), node)
	if err != nil {
		return 0, "dead"
	}
	return ms, "ok"
}

func XrayProbeNode(ctx context.Context, xrayBin, targetURL string, node config.Node) (int, error) {
	if xrayBin == "" {
		return 0, fmt.Errorf("xray binary not configured")
	}
	socksPort, err := freeTCPPort()
	if err != nil {
		return 0, err
	}
	cfgBytes, err := xray.GenerateProbeConfig(node, socksPort)
	if err != nil {
		return 0, err
	}

	tmp, err := os.CreateTemp("", "xray-probe-*.json")
	if err != nil {
		return 0, err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(cfgBytes); err != nil {
		tmp.Close()
		return 0, err
	}
	if err := tmp.Close(); err != nil {
		return 0, err
	}

	probeCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(probeCtx, xrayBin, "run", "-c", tmpPath)
	cmd.Dir = filepath.Dir(xrayBin)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("start xray probe: %w", err)
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()

	proxy := fmt.Sprintf("socks5h://127.0.0.1:%d", socksPort)
	deadline := time.Now().Add(12 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		if probeCtx.Err() != nil {
			if lastErr != nil {
				return 0, lastErr
			}
			return 0, probeCtx.Err()
		}
		ms, err := SOCKSProbeLatencyMs(probeCtx, proxy, targetURL)
		if err == nil {
			return ms, nil
		}
		lastErr = err
		time.Sleep(200 * time.Millisecond)
	}
	if lastErr != nil {
		return 0, lastErr
	}
	return 0, fmt.Errorf("xray probe timeout")
}

func SOCKSProbeLatencyMs(ctx context.Context, proxy, targetURL string) (int, error) {
	start := time.Now()
	_, err := SOCKSProbeAt(ctx, proxy, targetURL)
	if err != nil {
		return 0, err
	}
	return int(time.Since(start).Milliseconds()), nil
}

func freeTCPPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port, nil
}
