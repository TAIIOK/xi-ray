package service

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

type Watchdog struct {
	panel   *PanelService
	stop    chan struct{}
	mu      sync.Mutex
	running bool
	lastRun time.Time
}

func NewWatchdog(panel *PanelService) *Watchdog {
	return &Watchdog{panel: panel, stop: make(chan struct{})}
}

func (w *Watchdog) Start() {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.mu.Unlock()

	go w.loop()
}

func (w *Watchdog) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.running {
		return
	}
	close(w.stop)
	w.running = false
}

func (w *Watchdog) loop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.stop:
			return
		case <-ticker.C:
			cfg := w.panel.store.Get()
			if !cfg.Watchdog.Enabled {
				continue
			}
			interval := time.Duration(cfg.Watchdog.IntervalSec) * time.Second
			if interval < 15*time.Second {
				interval = 15 * time.Second
			}
			w.mu.Lock()
			shouldRun := w.lastRun.IsZero() || time.Since(w.lastRun) >= interval
			w.mu.Unlock()
			if !shouldRun {
				continue
			}
			w.runOnce(context.Background())
			w.mu.Lock()
			w.lastRun = time.Now()
			w.mu.Unlock()
		}
	}
}

func (w *Watchdog) runOnce(ctx context.Context) {
	status := w.panel.status.GetStatus(ctx)
	outage, reason := DetectWatchdogOutage(status.XrayRunning, status.VPNConnected, status.Observatory)

	if !outage {
		cfg := w.panel.store.Get()
		if w.panel.failOpen.IsActive() && cfg.FailOpen.RestoreOnRecoveryOrDefault() {
			if applyResult, err := w.panel.apply.Apply(ctx); err != nil {
				log.Printf("watchdog recovery apply error: %v", err)
			} else if applyResult != nil && !applyResult.OK {
				log.Printf("watchdog recovery apply: %s", applyResult.Message)
			}
		}
		if cfg.Watchdog.LastAlert != "" {
			_ = w.panel.store.Update(func(c *config.PanelConfig) error {
				c.Watchdog.LastAlert = ""
				return nil
			})
		}
		return
	}

	cfg := w.panel.store.Get()
	msg := fmt.Sprintf("[%s] Watchdog detected outage: %s", time.Now().Format(time.RFC3339), reason)
	log.Printf("watchdog: %s", msg)

	if cfg.Watchdog.RefreshSubscriptionsOnOutage && len(cfg.Subscriptions) > 0 {
		if _, err := w.panel.RefreshAllSubscriptionsManaged(ctx); err != nil {
			msg += fmt.Sprintf("; subscription refresh failed: %v", err)
		} else {
			msg += "; subscriptions refreshed"
		}
	}

	applyResult, err := w.panel.apply.Apply(ctx)
	if err != nil {
		msg += fmt.Sprintf("; apply error: %v", err)
	} else if applyResult != nil {
		msg += fmt.Sprintf("; apply: %s", applyResult.Message)
	}

	status = w.panel.status.GetStatus(ctx)
	outage, reason = DetectWatchdogOutage(status.XrayRunning, status.VPNConnected, status.Observatory)
	if outage && cfg.FailOpen.EnabledOrDefault() {
		if err := w.panel.failOpen.Enable(ctx); err != nil {
			msg += fmt.Sprintf("; fail-open error: %v", err)
		} else {
			msg += "; guest switched to direct internet (fail-open)"
			if reason != "" {
				msg += fmt.Sprintf(" (%s)", reason)
			}
		}
	}

	_ = w.panel.store.Update(func(c *config.PanelConfig) error {
		c.Watchdog.LastAlert = msg
		return nil
	})

	w.sendTelegram(cfg, msg)
}

func (w *Watchdog) sendTelegram(cfg config.PanelConfig, text string) {
	if cfg.Watchdog.TelegramBotToken == "" || cfg.Watchdog.TelegramChatID == "" {
		return
	}
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", cfg.Watchdog.TelegramBotToken)
	form := url.Values{}
	form.Set("chat_id", cfg.Watchdog.TelegramChatID)
	form.Set("text", text)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("watchdog telegram: %v", err)
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
}
