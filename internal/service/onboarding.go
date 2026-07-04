package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/taiiok/xiaomi-vless/internal/config"
	"github.com/taiiok/xiaomi-vless/internal/network"
	"github.com/taiiok/xiaomi-vless/internal/setup"
	"github.com/taiiok/xiaomi-vless/internal/subscription"
)

type OnboardingStatus struct {
	Required            bool                    `json:"required"`
	OnboardingCompleted bool                    `json:"onboarding_completed"`
	DefaultPassword     bool                    `json:"default_password"`
	Paths               config.Paths            `json:"paths"`
	PathChecks          []setup.PathCheck       `json:"path_checks"`
	USBMounts           []setup.USBMount        `json:"usb_mounts"`
	Ready               bool                    `json:"ready"`
	XrayVersion         string                  `json:"xray_version,omitempty"`
	HasNodes            bool                    `json:"has_nodes"`
	HasActiveSelection  bool                    `json:"has_active_selection"`
	VlessCount          int                     `json:"vless_count"`
	Nodes               []config.Node           `json:"nodes,omitempty"`
	Selection           config.Selection        `json:"selection"`
	GuestSubnet         string                  `json:"guest_subnet"`
	GuestNetwork        network.GuestNetworkStatus `json:"guest_network"`
}

func (p *PanelService) NeedsOnboarding() bool {
	cfg := p.store.Get()
	if p.store.IsDefaultPassword() {
		return true
	}
	return !cfg.Setup.OnboardingCompleted
}

func guestSubnetFromConfig(cfg config.PanelConfig) string {
	subnet := cfg.Network.GuestSubnet
	if subnet == "" {
		subnet = cfg.Iptables.GuestSubnet
	}
	if subnet == "" {
		subnet = config.DefaultGuestSubnet
	}
	return subnet
}

func (p *PanelService) CheckGuestNetwork(subnet string) network.GuestNetworkStatus {
	if subnet == "" {
		subnet = guestSubnetFromConfig(p.store.Get())
	}
	if norm, err := config.NormalizeGuestSubnet(subnet); err == nil {
		subnet = norm
	}
	return network.DetectGuest(subnet)
}

func (p *PanelService) UpdateOnboardingNetwork(guestSubnet string) (network.GuestNetworkStatus, error) {
	norm, err := config.NormalizeGuestSubnet(guestSubnet)
	if err != nil {
		return network.GuestNetworkStatus{}, err
	}
	if err := p.store.Update(func(cfg *config.PanelConfig) error {
		cfg.Network.GuestSubnet = norm
		cfg.Iptables.GuestSubnet = norm
		return nil
	}); err != nil {
		return network.GuestNetworkStatus{}, err
	}
	return p.CheckGuestNetwork(norm), nil
}

func (p *PanelService) GetOnboardingStatus() OnboardingStatus {
	cfg := p.store.Get()
	checks := setup.VerifyPaths(cfg.Paths)
	vless := subscription.FilterByProtocol(cfg.Nodes, "vless")
	guestSubnet := guestSubnetFromConfig(cfg)
	return OnboardingStatus{
		Required:            p.NeedsOnboarding(),
		OnboardingCompleted: cfg.Setup.OnboardingCompleted,
		DefaultPassword:     p.store.IsDefaultPassword(),
		Paths:               cfg.Paths,
		PathChecks:          checks,
		USBMounts:           setup.DiscoverUSBMounts(),
		Ready:               setup.AllChecksOK(checks),
		XrayVersion:         setup.XrayVersion(cfg.Paths.XrayBin),
		HasNodes:            len(cfg.Nodes) > 0,
		HasActiveSelection:  len(cfg.Selection.ActiveNodeIDs) > 0,
		VlessCount:          len(vless),
		Nodes:               cfg.Nodes,
		Selection:           cfg.Selection,
		GuestSubnet:         guestSubnet,
		GuestNetwork:        p.CheckGuestNetwork(guestSubnet),
	}
}

func (p *PanelService) UpdateOnboardingPaths(paths config.Paths) ([]setup.PathCheck, error) {
	if err := config.ValidatePaths(paths); err != nil {
		return nil, err
	}
	if err := p.store.Update(func(cfg *config.PanelConfig) error {
		cfg.Paths = paths
		if paths.PanelDataDir != "" {
			cfg.Logs = config.LogsForPanelHome(paths.PanelDataDir)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return setup.VerifyPaths(paths), nil
}

func (p *PanelService) VerifyOnboardingPaths() OnboardingStatus {
	return p.GetOnboardingStatus()
}

func (p *PanelService) SetupOnboardingEnvironment() setup.SetupResult {
	cfg := p.store.Get()
	_ = setup.EnsureSystemScripts(cfg.Paths)
	result := setup.RunSetup(cfg.Paths)
	return result
}

type DownloadXrayRequest struct {
	USBMount string        `json:"usb_mount,omitempty"`
	Paths    *config.Paths `json:"paths,omitempty"`
}

func (p *PanelService) DownloadXrayOnboarding(ctx context.Context, req DownloadXrayRequest) setup.XrayDownloadResult {
	cfg := p.store.Get()
	xrayBin := cfg.Paths.XrayBin
	if req.Paths != nil && req.Paths.XrayBin != "" {
		xrayBin = req.Paths.XrayBin
	}

	base, err := setup.ResolveInstallBase(setup.XrayDownloadOptions{
		USBMount: req.USBMount,
		XrayBin:  xrayBin,
	})
	if err != nil {
		return setup.XrayDownloadResult{Message: err.Error()}
	}

	result := setup.DownloadXray(ctx, base)
	paths := result.Paths
	if req.Paths != nil {
		if req.Paths.StartupScript != "" {
			paths.StartupScript = req.Paths.StartupScript
		}
		if req.Paths.IptablesScript != "" {
			paths.IptablesScript = req.Paths.IptablesScript
		}
		if req.Paths.PanelDataDir != "" {
			paths.PanelDataDir = req.Paths.PanelDataDir
		}
	} else {
		paths.StartupScript = cfg.Paths.StartupScript
		paths.IptablesScript = cfg.Paths.IptablesScript
		paths.PanelDataDir = cfg.Paths.PanelDataDir
	}

	_ = setup.EnsureSystemScripts(paths)
	if err := config.ValidatePaths(paths); err == nil {
		_ = p.store.Update(func(c *config.PanelConfig) error {
			c.Paths = paths
			return nil
		})
	}
	result.Paths = paths
	result.Checks = setup.VerifyPaths(paths)
	result.OK = setup.AllChecksOK(result.Checks)
	if result.OK && result.Message == "" {
		result.Message = "Xray установлен, пути обновлены"
	}
	return result
}

type CompleteOnboardingRequest struct {
	Username    string        `json:"username"`
	Password    string        `json:"password"`
	Paths       *config.Paths `json:"paths,omitempty"`
	GuestSubnet string        `json:"guest_subnet,omitempty"`
	Apply       bool          `json:"apply"`
}

type CompleteOnboardingResult struct {
	OK      bool         `json:"ok"`
	Message string       `json:"message"`
	Apply   *ApplyResult `json:"apply,omitempty"`
}

func (p *PanelService) CompleteOnboarding(ctx context.Context, req CompleteOnboardingRequest) (*CompleteOnboardingResult, error) {
	if req.Password == "" {
		return nil, fmt.Errorf("password required")
	}
	if len(req.Password) < 6 {
		return nil, fmt.Errorf("password must be at least 6 characters")
	}
	if req.Paths != nil {
		if _, err := p.UpdateOnboardingPaths(*req.Paths); err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(req.GuestSubnet) != "" {
		if _, err := p.UpdateOnboardingNetwork(req.GuestSubnet); err != nil {
			return nil, err
		}
	}

	username := req.Username
	if username == "" {
		username = p.store.Get().Auth.Username
	}
	if err := p.store.SetPassword(username, req.Password); err != nil {
		return nil, err
	}
	if err := p.store.Update(func(cfg *config.PanelConfig) error {
		cfg.Setup.OnboardingCompleted = true
		sanitizeSelection(cfg)
		return nil
	}); err != nil {
		return nil, err
	}

	result := &CompleteOnboardingResult{OK: true, Message: "onboarding completed"}
	if req.Apply {
		applyResult, err := p.apply.Apply(ctx)
		if err != nil {
			return nil, err
		}
		result.Apply = applyResult
		result.OK = applyResult.OK
		result.Message = applyResult.Message
	}
	return result, nil
}

func (p *PanelService) SkipOnboarding() error {
	return p.store.Update(func(cfg *config.PanelConfig) error {
		cfg.Setup.OnboardingCompleted = true
		return nil
	})
}

type ImportOnboardingRequest struct {
	Input string `json:"input"`
	Name  string `json:"name,omitempty"`
}

type ImportOnboardingResult struct {
	Kind         string               `json:"kind"`
	Message      string               `json:"message"`
	TotalNodes   int                  `json:"total_nodes"`
	VlessCount   int                  `json:"vless_count"`
	Nodes        []config.Node        `json:"nodes"`
	VlessNodes   []config.Node        `json:"vless_nodes"`
	Subscription *config.Subscription `json:"subscription,omitempty"`
	Selection    config.Selection     `json:"selection"`
}

func (p *PanelService) ImportOnboardingInput(ctx context.Context, req ImportOnboardingRequest) (*ImportOnboardingResult, error) {
	input := strings.TrimSpace(req.Input)
	if input == "" {
		return nil, fmt.Errorf("укажите ссылку или URL подписки")
	}

	result := &ImportOnboardingResult{}

	switch {
	case subscription.IsSubscriptionURL(input):
		name := strings.TrimSpace(req.Name)
		if name == "" {
			name = "Подписка"
		}
		sub, nodes, err := p.AddSubscription(ctx, name, input)
		if err != nil {
			return nil, err
		}
		result.Kind = "subscription"
		result.Subscription = &sub
		result.Nodes = nodes
		result.TotalNodes = len(nodes)
		result.VlessNodes = subscription.FilterByProtocol(nodes, "vless")
		result.VlessCount = len(result.VlessNodes)
		if result.VlessCount == 0 {
			result.Message = fmt.Sprintf("Подписка загружена: %d сервер(ов), VLESS не найден", result.TotalNodes)
		} else {
			result.Message = fmt.Sprintf("Подписка загружена: %d сервер(ов), %d VLESS", result.TotalNodes, result.VlessCount)
		}

	case subscription.IsProxyLink(input):
		node, err := p.AddManualNode(input)
		if err != nil {
			return nil, err
		}
		result.Kind = "link"
		result.Nodes = []config.Node{node}
		result.TotalNodes = 1
		result.VlessNodes = subscription.FilterByProtocol(result.Nodes, "vless")
		result.VlessCount = len(result.VlessNodes)
		if strings.EqualFold(node.Protocol, "vless") {
			result.Message = "VLESS сервер добавлен"
		} else {
			result.Message = fmt.Sprintf("Сервер добавлен (%s), VLESS в подписке не найден", node.Protocol)
		}

	default:
		return nil, fmt.Errorf("неизвестный формат: укажите https://… подписку или vless://… ссылку")
	}

	cfg := p.store.Get()
	result.Nodes = cfg.Nodes
	result.VlessNodes = vlessNodesFromStore(cfg.Nodes)
	result.VlessCount = len(result.VlessNodes)

	if result.VlessCount > 0 {
		needsFix := false
		byID := map[string]struct{}{}
		for _, n := range cfg.Nodes {
			byID[n.ID] = struct{}{}
		}
		for _, id := range cfg.Selection.ActiveNodeIDs {
			if _, ok := byID[id]; ok {
				needsFix = false
				break
			}
			needsFix = true
		}
		if len(cfg.Selection.ActiveNodeIDs) == 0 {
			needsFix = true
		}
		if needsFix {
			_ = p.store.Update(func(c *config.PanelConfig) error {
				sanitizeSelection(c)
				return nil
			})
		}
	}

	result.Selection = p.store.Get().Selection
	return result, nil
}

func (p *PanelService) SaveOnboardingSelection(mode string, activeIDs []string) error {
	if mode == "" {
		mode = "single"
	}
	if len(activeIDs) == 0 {
		return fmt.Errorf("выберите хотя бы один VLESS сервер")
	}
	return p.UpdateSelection(mode, activeIDs, activeIDs)
}
