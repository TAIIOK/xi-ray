package service

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

type SubscriptionScheduler struct {
	panel   *PanelService
	stop    chan struct{}
	mu      sync.Mutex
	running bool
}

func NewSubscriptionScheduler(panel *PanelService) *SubscriptionScheduler {
	return &SubscriptionScheduler{panel: panel, stop: make(chan struct{})}
}

func (s *SubscriptionScheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()
	go s.loop()
}

func (s *SubscriptionScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	close(s.stop)
	s.running = false
}

func (s *SubscriptionScheduler) loop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.runOnce(context.Background())
		}
	}
}

func (s *SubscriptionScheduler) runOnce(ctx context.Context) {
	cfg := s.panel.store.Get()
	pol := cfg.SubscriptionsPolicy
	if !pol.AutoRefreshEnabled || len(cfg.Subscriptions) == 0 {
		return
	}

	interval := time.Duration(pol.AutoRefreshIntervalMin) * time.Minute
	if interval < 5*time.Minute {
		interval = 5 * time.Minute
	}
	if !pol.LastAutoRefreshAt.IsZero() && time.Since(pol.LastAutoRefreshAt) < interval {
		return
	}

	result, err := s.panel.RefreshAllSubscriptionsManaged(ctx)
	if err != nil {
		log.Printf("subscription auto-refresh: %v", err)
	}
	_ = s.panel.store.Update(func(c *config.PanelConfig) error {
		c.SubscriptionsPolicy.LastAutoRefreshAt = time.Now()
		return nil
	})

	if result.Refreshed > 0 {
		log.Printf("subscription auto-refresh: refreshed %d, selection_changed=%v, apply_ok=%v",
			result.Refreshed, result.SelectionChanged, result.ApplyOK())
	}
}
