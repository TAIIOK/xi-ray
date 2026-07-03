package xray

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	observatorycmd "github.com/xtls/xray-core/app/observatory/command"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const DefaultAPIAddr = "127.0.0.1:10085"

type APIClient struct {
	Addr string
}

func NewAPIClient(addr string) *APIClient {
	if addr == "" {
		addr = DefaultAPIAddr
	}
	return &APIClient{Addr: addr}
}

type LiveOutboundStatus struct {
	OutboundTag string `json:"outbound_tag"`
	Alive       bool   `json:"alive"`
	DelayMs     int64  `json:"delay_ms"`
	LastError   string `json:"last_error,omitempty"`
	Available   bool   `json:"available"`
}

func (c *APIClient) ValidateConfig(ctx context.Context, xrayBin, path string) error {
	cmd := exec.CommandContext(ctx, xrayBin, "run", "-test", "-c", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *APIClient) GetOutboundStatuses(ctx context.Context) ([]LiveOutboundStatus, error) {
	dialCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		dialCtx,
		c.Addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("dial xray api: %w", err)
	}
	defer conn.Close()

	client := observatorycmd.NewObservatoryServiceClient(conn)
	resp, err := client.GetOutboundStatus(ctx, &observatorycmd.GetOutboundStatusRequest{})
	if err != nil {
		return nil, fmt.Errorf("observatory api: %w", err)
	}
	if resp.GetStatus() == nil {
		return []LiveOutboundStatus{}, nil
	}

	out := make([]LiveOutboundStatus, 0, len(resp.GetStatus().GetStatus()))
	for _, st := range resp.GetStatus().GetStatus() {
		out = append(out, LiveOutboundStatus{
			OutboundTag: st.GetOutboundTag(),
			Alive:       st.GetAlive(),
			DelayMs:     st.GetDelay(),
			LastError:   st.GetLastErrorReason(),
			Available:   true,
		})
	}
	return out, nil
}

func PickBestAlive(statuses []LiveOutboundStatus) string {
	var best string
	var bestDelay int64 = -1
	for _, st := range statuses {
		if !st.Alive {
			continue
		}
		if best == "" || (st.DelayMs >= 0 && (bestDelay < 0 || st.DelayMs < bestDelay)) {
			best = st.OutboundTag
			bestDelay = st.DelayMs
		}
	}
	return best
}
