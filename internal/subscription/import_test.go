package subscription

import (
	"testing"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

func TestIsSubscriptionURL(t *testing.T) {
	if !IsSubscriptionURL("https://example.com/sub") {
		t.Fatal("expected https URL")
	}
	if IsSubscriptionURL("vless://uuid@host:443") {
		t.Fatal("vless link is not subscription URL")
	}
}

func TestFilterByProtocol(t *testing.T) {
	nodes := []config.Node{
		{ID: "1", Protocol: "vless", Name: "a"},
		{ID: "2", Protocol: "vmess", Name: "b"},
		{ID: "3", Protocol: "VLESS", Name: "c"},
	}
	vless := FilterByProtocol(nodes, "vless")
	if len(vless) != 2 {
		t.Fatalf("got %d vless nodes", len(vless))
	}
}
