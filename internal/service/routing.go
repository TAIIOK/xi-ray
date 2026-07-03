package service

import (
	"context"
	"fmt"

	"github.com/taiiok/xiaomi-vless/internal/config"
	"github.com/taiiok/xiaomi-vless/internal/xray"
)

type RoutingResponse struct {
	Routing       config.Routing     `json:"routing"`
	Preview       []xray.PreviewRule `json:"preview"`
	UseBalancer   bool               `json:"use_balancer"`
	SelectionMode string             `json:"selection_mode"`
	ActiveNodes   int                `json:"active_nodes"`
}

func (p *PanelService) GetRouting() (*RoutingResponse, error) {
	cfg := p.store.Get()
	return p.buildRoutingResponse(cfg)
}

func (p *PanelService) buildRoutingResponse(cfg config.PanelConfig) (*RoutingResponse, error) {
	nodes, err := xray.ActiveNodes(cfg)
	if err != nil {
		nodes = []config.Node{}
	}
	var proxyTags []string
	for _, n := range nodes {
		proxyTags = append(proxyTags, xray.OutboundTag(n.ID))
	}
	useBalancer := cfg.Selection.Mode == "multi" && len(proxyTags) > 1 && cfg.Observatory.Enabled

	return &RoutingResponse{
		Routing:       cfg.Routing,
		Preview:       xray.BuildRoutingPreview(cfg, proxyTags, nodes),
		UseBalancer:   useBalancer,
		SelectionMode: cfg.Selection.Mode,
		ActiveNodes:   len(cfg.Selection.ActiveNodeIDs),
	}, nil
}

func (p *PanelService) UpdateRouting(routing config.Routing) error {
	routing.Normalize()
	if err := config.ValidateRouting(routing); err != nil {
		return err
	}
	return p.store.Update(func(cfg *config.PanelConfig) error {
		cfg.Routing = routing
		return nil
	})
}

type RoutingSaveResult struct {
	OK      bool               `json:"ok"`
	Message string             `json:"message"`
	Routing config.Routing     `json:"routing"`
	Preview []xray.PreviewRule `json:"preview"`
	Apply   *ApplyResult       `json:"apply,omitempty"`
}

func (p *PanelService) UpdateRoutingWithApply(ctx context.Context, routing config.Routing, apply bool) (*RoutingSaveResult, error) {
	if err := p.UpdateRouting(routing); err != nil {
		return nil, err
	}
	resp, err := p.GetRouting()
	if err != nil {
		return nil, err
	}
	result := &RoutingSaveResult{
		OK:      true,
		Message: "routing saved",
		Routing: resp.Routing,
		Preview: resp.Preview,
	}
	if !apply {
		return result, nil
	}
	applyResult, err := p.apply.Apply(ctx)
	if err != nil {
		return nil, err
	}
	result.Apply = applyResult
	result.OK = applyResult.OK
	result.Message = fmt.Sprintf("routing saved; %s", applyResult.Message)
	return result, nil
}
