package subscription

import (
	"encoding/base64"
	"testing"
)

func TestParseVMess(t *testing.T) {
	payload := base64.StdEncoding.EncodeToString([]byte(`{
		"v":"2","ps":"test-vmess","add":"vm.example.com","port":"443",
		"id":"00000000-0000-0000-0000-000000000099","aid":"0","net":"ws","tls":"tls","sni":"vm.example.com"
	}`))
	raw := "vmess://" + payload
	node, err := ParseLink(raw)
	if err != nil {
		t.Fatal(err)
	}
	if node.Protocol != "vmess" {
		t.Fatalf("protocol: %s", node.Protocol)
	}
	if node.Address != "vm.example.com" || node.Port != 443 {
		t.Fatalf("addr/port: %s:%d", node.Address, node.Port)
	}
	if node.Security != "tls" {
		t.Fatalf("security: %s", node.Security)
	}
}

func TestParseTrojan(t *testing.T) {
	raw := "trojan://secret-pass@trojan.example.com:443?sni=trojan.example.com#MyTrojan"
	node, err := ParseLink(raw)
	if err != nil {
		t.Fatal(err)
	}
	if node.Protocol != "trojan" {
		t.Fatalf("protocol: %s", node.Protocol)
	}
	if node.Password != "secret-pass" {
		t.Fatalf("password: %s", node.Password)
	}
	if node.Name != "MyTrojan" {
		t.Fatalf("name: %s", node.Name)
	}
}

func TestParseShadowsocks(t *testing.T) {
	raw := "ss://YWVzLTI1Ni1nY206c2VjcmV0@ss.example.com:8388#SSNode"
	node, err := ParseLink(raw)
	if err != nil {
		t.Fatal(err)
	}
	if node.Protocol != "shadowsocks" {
		t.Fatalf("protocol: %s", node.Protocol)
	}
	if node.Method != "aes-256-gcm" || node.Password != "secret" {
		t.Fatalf("method/password: %s / %s", node.Method, node.Password)
	}
	if node.Address != "ss.example.com" || node.Port != 8388 {
		t.Fatalf("host: %s:%d", node.Address, node.Port)
	}
}

func TestParseShadowsocksPlain(t *testing.T) {
	raw := "ss://aes-128-gcm:hello@127.0.0.1:9999#local"
	node, err := ParseLink(raw)
	if err != nil {
		t.Fatal(err)
	}
	if node.Method != "aes-128-gcm" || node.Password != "hello" {
		t.Fatalf("creds: %s/%s", node.Method, node.Password)
	}
	if node.Port != 9999 {
		t.Fatalf("port: %d", node.Port)
	}
}

func TestParseVLESSXHTTP(t *testing.T) {
	raw := "vless://00000000-0000-0000-0000-000000000001@host.example.com:443?" +
		"security=tls&sni=host.example.com&fp=chrome&type=xhttp&path=%2Fxhttp&host=host.example.com&mode=auto" +
		"&extra=%7B%22xmux%22%3A%7B%22maxConcurrency%22%3A%2216-32%22%7D%7D#xhttp-node"
	node, err := ParseLink(raw)
	if err != nil {
		t.Fatal(err)
	}
	if node.Network != "xhttp" {
		t.Fatalf("network: %s", node.Network)
	}
	if node.Path != "/xhttp" || node.XHTTPMode != "auto" {
		t.Fatalf("path/mode: %s / %s", node.Path, node.XHTTPMode)
	}
	if node.XHTTPExtra == nil || node.XHTTPExtra["xmux"] == nil {
		t.Fatalf("extra missing: %#v", node.XHTTPExtra)
	}
}
