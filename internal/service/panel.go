package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/taiiok/xiaomi-vless/internal/config"
	"github.com/taiiok/xiaomi-vless/internal/subscription"
)

type PanelService struct {
	store    *config.Store
	fetcher  *subscription.Fetcher
	apply    *ApplyService
	status   *StatusService
	failOpen *FailOpenService
}

func NewPanelService(store *config.Store) *PanelService {
	failOpen := NewFailOpenService(store)
	return &PanelService{
		store:    store,
		fetcher:  subscription.NewFetcher(),
		failOpen: failOpen,
		apply:    NewApplyService(store, failOpen),
		status:  NewStatusService(store, failOpen),
	}
}

func (p *PanelService) Store() *config.Store      { return p.store }
func (p *PanelService) Apply() *ApplyService      { return p.apply }
func (p *PanelService) Status() *StatusService    { return p.status }
func (p *PanelService) FailOpen() *FailOpenService { return p.failOpen }

type SubscriptionRefreshResult struct {
	Nodes            []config.Node `json:"nodes"`
	ListChanged      bool          `json:"list_changed"`
	SelectionChanged bool          `json:"selection_changed"`
	SelectedNodeIDs  []string      `json:"selected_node_ids,omitempty"`
	Apply            *ApplyResult  `json:"apply,omitempty"`
}

type SubscriptionRefreshBatchResult struct {
	Refreshed        int          `json:"refreshed"`
	ListChanged      bool         `json:"list_changed"`
	SelectionChanged bool         `json:"selection_changed"`
	SelectedNodeIDs  []string     `json:"selected_node_ids,omitempty"`
	Apply            *ApplyResult `json:"apply,omitempty"`
}

func (r SubscriptionRefreshBatchResult) ApplyOK() bool {
	return r.Apply != nil && r.Apply.OK
}

func (p *PanelService) AddSubscription(ctx context.Context, name, url string) (config.Subscription, []config.Node, error) {
	sub := subscription.NewSubscription(name, url)
	nodes, err := p.fetcher.FetchSubscription(url)
	if err != nil {
		return config.Subscription{}, nil, err
	}
	nodes = subscription.StampSubscriptionNodes(nodes, sub)

	if err := p.store.Update(func(cfg *config.PanelConfig) error {
		cfg.Subscriptions = append(cfg.Subscriptions, sub)
		cfg.Nodes = subscription.MergeNodes(cfg.Nodes, nodes)
		if len(cfg.Selection.ActiveNodeIDs) == 0 {
			reselectAfterSubscriptionUpdate(cfg, sub.ID, map[string]struct{}{}, cfg.SubscriptionsPolicy.ReselectStrategy)
		}
		return nil
	}); err != nil {
		return config.Subscription{}, nil, err
	}
	stored := nodesBySubscription(p.store.Get().Nodes, sub.ID)
	return sub, stored, nil
}

func (p *PanelService) RefreshSubscription(ctx context.Context, id string) ([]config.Node, error) {
	result, err := p.RefreshSubscriptionManaged(ctx, id)
	if err != nil {
		return nil, err
	}
	return result.Nodes, nil
}

func (p *PanelService) RefreshSubscriptionManaged(ctx context.Context, id string) (*SubscriptionRefreshResult, error) {
	result, err := p.refreshSubscriptionStore(ctx, id)
	if err != nil {
		return nil, err
	}
	p.maybeApplyAfterRefresh(ctx, result)
	return result, nil
}

func (p *PanelService) refreshSubscriptionStore(ctx context.Context, id string) (*SubscriptionRefreshResult, error) {
	cfg := p.store.Get()
	oldHashes := nodeHashSet(nodesBySubscription(cfg.Nodes, id))
	policy := cfg.SubscriptionsPolicy

	var sub config.Subscription
	found := false
	for _, s := range cfg.Subscriptions {
		if s.ID == id {
			sub = s
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("subscription not found")
	}

	nodes, err := p.fetcher.FetchSubscription(sub.URL)
	if err != nil {
		return nil, err
	}
	sub.UpdatedAt = time.Now()
	nodes = subscription.StampSubscriptionNodes(nodes, sub)

	listChanged := false
	selectionChanged := false
	var selected []string

	if err := p.store.Update(func(cfg *config.PanelConfig) error {
		for i, s := range cfg.Subscriptions {
			if s.ID == id {
				cfg.Subscriptions[i] = sub
				break
			}
		}
		cfg.Nodes = subscription.RemoveNodesBySubscription(cfg.Nodes, id)
		cfg.Nodes = subscription.MergeNodes(cfg.Nodes, nodes)
		listChanged = !hashSetsEqual(oldHashes, nodeHashSet(nodesBySubscription(cfg.Nodes, id)))
		selectionChanged = reselectAfterSubscriptionUpdate(cfg, id, oldHashes, policy.ReselectStrategy)
		selected = append([]string(nil), cfg.Selection.ActiveNodeIDs...)
		return nil
	}); err != nil {
		return nil, err
	}

	result := &SubscriptionRefreshResult{
		Nodes:            nodesBySubscription(p.store.Get().Nodes, id),
		ListChanged:      listChanged,
		SelectionChanged: selectionChanged,
		SelectedNodeIDs:  selected,
	}
	return result, nil
}

func (p *PanelService) maybeApplyAfterRefresh(ctx context.Context, result *SubscriptionRefreshResult) {
	if result == nil {
		return
	}
	policy := p.store.Get().SubscriptionsPolicy
	if !policy.AutoApplyOnChange || (!result.SelectionChanged && !result.ListChanged) {
		return
	}
	applyResult, err := p.apply.Apply(ctx)
	if err == nil {
		result.Apply = applyResult
	}
}

func (p *PanelService) RefreshAllSubscriptions(ctx context.Context) error {
	_, err := p.RefreshAllSubscriptionsManaged(ctx)
	return err
}

func (p *PanelService) RefreshAllSubscriptionsManaged(ctx context.Context) (*SubscriptionRefreshBatchResult, error) {
	cfg := p.store.Get()
	batch := &SubscriptionRefreshBatchResult{}
	var firstErr error
	needsApply := false

	for _, sub := range cfg.Subscriptions {
		result, err := p.refreshSubscriptionStore(ctx, sub.ID)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		batch.Refreshed++
		if result.ListChanged {
			batch.ListChanged = true
			needsApply = true
		}
		if result.SelectionChanged {
			batch.SelectionChanged = true
			batch.SelectedNodeIDs = result.SelectedNodeIDs
			needsApply = true
		}
	}

	if needsApply && cfg.SubscriptionsPolicy.AutoApplyOnChange {
		applyResult, err := p.apply.Apply(ctx)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		batch.Apply = applyResult
	}
	return batch, firstErr
}

func (p *PanelService) DeleteSubscription(id string) error {
	return p.store.Update(func(cfg *config.PanelConfig) error {
		subs := make([]config.Subscription, 0, len(cfg.Subscriptions))
		for _, s := range cfg.Subscriptions {
			if s.ID == id {
				continue
			}
			subs = append(subs, s)
		}
		cfg.Subscriptions = subs
		cfg.Nodes = subscription.RemoveNodesBySubscription(cfg.Nodes, id)

		filtered := make([]string, 0)
		for _, nid := range cfg.Selection.ActiveNodeIDs {
			if _, ok := nodeByID(cfg.Nodes, nid); ok {
				filtered = append(filtered, nid)
			}
		}
		cfg.Selection.ActiveNodeIDs = filtered
		return nil
	})
}

func (p *PanelService) AddManualNode(link string) (config.Node, error) {
	parsed, err := subscription.ParseLink(link)
	if err != nil {
		return config.Node{}, err
	}
	node := subscription.NewManualNode(link, parsed)
	hash := node.Hash

	if err := p.store.Update(func(cfg *config.PanelConfig) error {
		cfg.Nodes = subscription.MergeNodes(cfg.Nodes, []config.Node{node})
		return nil
	}); err != nil {
		return config.Node{}, err
	}
	if merged, ok := nodeByHash(p.store.Get().Nodes, hash); ok {
		return merged, nil
	}
	return node, nil
}

func (p *PanelService) UpdateSelection(mode string, activeIDs, fallbackOrder []string) error {
	if mode != "single" && mode != "multi" {
		return fmt.Errorf("invalid mode")
	}
	if mode == "single" && len(activeIDs) > 1 {
		activeIDs = activeIDs[:1]
	}
	if len(activeIDs) == 0 {
		return fmt.Errorf("select at least one node")
	}

	return p.store.Update(func(cfg *config.PanelConfig) error {
		for _, id := range activeIDs {
			if _, ok := nodeByID(cfg.Nodes, id); !ok {
				return fmt.Errorf("node not found: %s", id)
			}
		}
		cfg.Selection.Mode = mode
		cfg.Selection.ActiveNodeIDs = activeIDs
		if len(fallbackOrder) == 0 {
			cfg.Selection.FallbackOrder = activeIDs
		} else {
			cfg.Selection.FallbackOrder = fallbackOrder
		}
		return nil
	})
}

type SelectionResult struct {
	OK      bool         `json:"ok"`
	Message string       `json:"message"`
	Apply   *ApplyResult `json:"apply,omitempty"`
}

func (p *PanelService) UpdateSelectionWithApply(ctx context.Context, mode string, activeIDs, fallbackOrder []string, apply bool) (*SelectionResult, error) {
	if err := p.UpdateSelection(mode, activeIDs, fallbackOrder); err != nil {
		return nil, err
	}
	result := &SelectionResult{OK: true, Message: "selection saved"}
	if !apply {
		return result, nil
	}
	applyResult, err := p.apply.Apply(ctx)
	if err != nil {
		return nil, err
	}
	result.Apply = applyResult
	result.OK = applyResult.OK
	result.Message = applyResult.Message
	return result, nil
}

func (p *PanelService) UpdateSettings(paths config.Paths, networkCfg config.Network, iptables config.Iptables, obs config.Observatory, watchdog config.Watchdog, failOpen config.FailOpen, subsPolicy config.SubscriptionsPolicy, logs config.Logs, routing config.Routing) error {
	if err := config.ValidatePaths(paths); err != nil {
		return err
	}
	subnet := networkCfg.GuestSubnet
	if subnet == "" {
		subnet = iptables.GuestSubnet
	}
	if norm, err := config.NormalizeGuestSubnet(subnet); err != nil {
		return err
	} else {
		networkCfg.GuestSubnet = norm
		iptables.GuestSubnet = norm
	}
	skipRouting := routing.IsEmptyPayload()
	if !skipRouting {
		routing.Normalize()
		if err := config.ValidateRouting(routing); err != nil {
			return err
		}
	}
	return p.store.Update(func(cfg *config.PanelConfig) error {
		cfg.Paths = paths
		cfg.Network = networkCfg
		cfg.Iptables = iptables
		if cfg.Iptables.GuestSubnet == "" {
			cfg.Iptables.GuestSubnet = networkCfg.GuestSubnet
		}
		if cfg.Network.GuestSubnet == "" {
			cfg.Network.GuestSubnet = iptables.GuestSubnet
		}
		cfg.Observatory = obs
		prevLastAlert := cfg.Watchdog.LastAlert
		cfg.Watchdog = watchdog
		if watchdog.LastAlert == "" && prevLastAlert != "" {
			cfg.Watchdog.LastAlert = prevLastAlert
		}
		cfg.FailOpen = failOpen
		cfg.SubscriptionsPolicy = subsPolicy
		cfg.Logs = logs
		if !skipRouting {
			cfg.Routing = routing
		}
		cfg.Normalize()
		return nil
	})
}

func (p *PanelService) DismissWatchdogAlert() error {
	return p.store.Update(func(cfg *config.PanelConfig) error {
		cfg.Watchdog.LastAlert = ""
		return nil
	})
}

func (p *PanelService) DeleteNode(id string) error {
	return p.store.Update(func(cfg *config.PanelConfig) error {
		nodes := make([]config.Node, 0, len(cfg.Nodes))
		for _, n := range cfg.Nodes {
			if n.ID == id {
				continue
			}
			nodes = append(nodes, n)
		}
		cfg.Nodes = nodes
		cfg.Selection.ActiveNodeIDs = filterActive(cfg.Selection.ActiveNodeIDs, id)
		cfg.Selection.FallbackOrder = filterActive(cfg.Selection.FallbackOrder, id)
		return nil
	})
}

func nodeByID(nodes []config.Node, id string) (config.Node, bool) {
	for _, n := range nodes {
		if n.ID == id {
			return n, true
		}
	}
	return config.Node{}, false
}

func filterActive(ids []string, remove string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != remove {
			out = append(out, id)
		}
	}
	return out
}

func (p *PanelService) NewID() string {
	return uuid.NewString()
}
