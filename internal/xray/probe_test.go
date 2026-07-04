package xray

import (
	"encoding/json"
	"testing"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

func TestGenerateProbeConfig(t *testing.T) {
	node := config.Node{
		ID: "abc12345-0000", Protocol: "vless", UUID: "uuid",
		Address: "example.com", Port: 443, Security: "tls", Network: "tcp",
	}
	raw, err := GenerateProbeConfig(node, 19080)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	outbounds, ok := doc["outbounds"].([]any)
	if !ok || len(outbounds) < 1 {
		t.Fatalf("expected outbounds, got %#v", doc["outbounds"])
	}
	first, ok := outbounds[0].(map[string]any)
	if !ok || first["tag"] != probeOutboundTag {
		t.Fatalf("probe outbound tag: %#v", first)
	}
	inbounds, ok := doc["inbounds"].([]any)
	if !ok || len(inbounds) != 1 {
		t.Fatalf("expected one inbound")
	}
	in, ok := inbounds[0].(map[string]any)
	if !ok || in["port"].(float64) != 19080 {
		t.Fatalf("socks port: %#v", in)
	}
}
