package subscription

import (
	"testing"
)

const sampleVLESS = "vless://00000000-0000-0000-0000-000000000001@host.example.com:443?" +
	"flow=xtls-rprx-vision&type=tcp&headerType=none&security=reality&fp=edge&sni=www.example.com" +
	"&pbk=test-public-key&sid=abcd1234&spx=/path#test-node"

func TestParseVLESSReality(t *testing.T) {
	node, err := ParseLink(sampleVLESS)
	if err != nil {
		t.Fatal(err)
	}
	if node.Address != "host.example.com" {
		t.Fatalf("address: %s", node.Address)
	}
	if node.Port != 443 {
		t.Fatalf("port: %d", node.Port)
	}
	if node.Security != "reality" {
		t.Fatalf("security: %s", node.Security)
	}
	if node.UUID != "00000000-0000-0000-0000-000000000001" {
		t.Fatalf("uuid: %s", node.UUID)
	}
	if node.Name != "test-node" {
		t.Fatalf("name: %s", node.Name)
	}
}

func TestParseSubscriptionBody(t *testing.T) {
	body := sampleVLESS
	nodes, err := ParseSubscriptionBody([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("nodes: %d", len(nodes))
	}
}
