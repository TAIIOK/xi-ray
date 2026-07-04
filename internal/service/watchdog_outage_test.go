package service

import "testing"

func TestDetectWatchdogOutage(t *testing.T) {
	t.Run("xray down", func(t *testing.T) {
		active, reason := DetectWatchdogOutage(false, false, ObservatoryStatus{})
		if !active || reason != "xray not running" {
			t.Fatalf("got active=%v reason=%q", active, reason)
		}
	})

	t.Run("vpn ok", func(t *testing.T) {
		active, _ := DetectWatchdogOutage(true, true, ObservatoryStatus{})
		if active {
			t.Fatal("expected no outage when vpn connected")
		}
	})

	t.Run("vpn down all dead", func(t *testing.T) {
		obs := ObservatoryStatus{Nodes: []NodeHealth{{Alive: false}, {Alive: false}}}
		active, reason := DetectWatchdogOutage(true, false, obs)
		if !active || reason == "" {
			t.Fatalf("got active=%v reason=%q", active, reason)
		}
	})

	t.Run("vpn down one alive", func(t *testing.T) {
		obs := ObservatoryStatus{Nodes: []NodeHealth{{Alive: true}}}
		active, _ := DetectWatchdogOutage(true, false, obs)
		if active {
			t.Fatal("expected no watchdog outage when an outbound is alive")
		}
	})
}
