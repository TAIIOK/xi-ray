package service

import (
	"context"

	"github.com/taiiok/xiaomi-vless/internal/setup"
)

type XrayStatusResponse struct {
	Version string `json:"version,omitempty"`
	Path    string `json:"path"`
	OK      bool   `json:"ok"`
}

func (p *PanelService) XrayStatus() XrayStatusResponse {
	cfg := p.store.Get()
	path := cfg.Paths.XrayBin
	ver := setup.XrayVersion(path)
	return XrayStatusResponse{
		Version: ver,
		Path:    path,
		OK:      ver != "",
	}
}

type UpdateXrayRequest struct {
	Restart bool `json:"restart,omitempty"`
}

type UpdateXrayResponse struct {
	setup.XrayDownloadResult
	Restart *ApplyResult `json:"restart,omitempty"`
}

func (p *PanelService) UpdateXray(ctx context.Context, req UpdateXrayRequest) UpdateXrayResponse {
	dlReq := DownloadXrayRequest{}
	result := p.DownloadXrayOnboarding(ctx, dlReq)
	resp := UpdateXrayResponse{XrayDownloadResult: result}
	if !result.OK || !req.Restart {
		return resp
	}
	applyResult, err := p.apply.Apply(ctx)
	if err != nil {
		resp.Restart = &ApplyResult{OK: false, Message: err.Error()}
		return resp
	}
	resp.Restart = applyResult
	return resp
}
