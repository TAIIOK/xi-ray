package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/taiiok/xiaomi-vless/internal/config"
	"github.com/taiiok/xiaomi-vless/internal/service"
	"github.com/taiiok/xiaomi-vless/internal/update"
)

type testEnv struct {
	server *httptest.Server
	store  *config.Store
	client *http.Client
}

func newRoutingTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "panel.json")
	store, err := config.NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetPassword("admin", "secret"); err != nil {
		t.Fatal(err)
	}
	nodeID := "node-12345678"
	if err := store.Update(func(cfg *config.PanelConfig) error {
		cfg.Setup.OnboardingCompleted = true
		cfg.Nodes = []config.Node{{
			ID: nodeID, Name: "test", Protocol: "vless",
			Address: "vpn.example.com", Port: 443,
			UUID: "00000000-0000-0000-0000-000000000001",
			Security: "tls", Network: "tcp", SNI: "vpn.example.com",
		}}
		cfg.Selection = config.Selection{
			Mode:          "single",
			ActiveNodeIDs: []string{nodeID},
			FallbackOrder: []string{nodeID},
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	panel := service.NewPanelService(store)
	upd := update.NewService(dir, path, store.Get)
	srv := New(panel, upd)
	ts := httptest.NewServer(srv.Router())

	jar := &cookieJar{}
	client := &http.Client{
		Jar:           jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	loginReq, err := http.NewRequest(http.MethodPost, ts.URL+"/login", strings.NewReader("username=admin&password=secret"))
	if err != nil {
		t.Fatal(err)
	}
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginResp, err := client.Do(loginReq)
	if err != nil {
		t.Fatal(err)
	}
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login status: %d", loginResp.StatusCode)
	}

	return &testEnv{
		server: ts,
		store:  store,
		client: client,
	}
}

type cookieJar struct {
	cookies []*http.Cookie
}

func (j *cookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	j.cookies = cookies
}

func (j *cookieJar) Cookies(u *url.URL) []*http.Cookie {
	return j.cookies
}

func (e *testEnv) close() { e.server.Close() }

func (e *testEnv) doJSON(method, path string, body any) (*http.Response, map[string]any) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			panic(err)
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, e.server.URL+path, r)
	if err != nil {
		panic(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := e.client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

func sampleRoutingPayload() map[string]any {
	return map[string]any{
		"domain_strategy":      "IPIfNonMatch",
		"rule_order":           []string{"direct", "block", "proxy"},
		"default_guest_action": "proxy",
		"bypass_private":       true,
		"bypass_vpn_hosts":     true,
		"rules": []map[string]any{
			{"id": "cn", "name": "CN sites", "action": "direct", "domains": []string{"geosite:cn"}, "enabled": true},
			{"id": "ads", "name": "Block ads", "action": "block", "domains": []string{"geosite:category-ads-all"}, "enabled": true},
		},
	}
}

func TestAPIRoutingGET(t *testing.T) {
	env := newRoutingTestEnv(t)
	defer env.close()

	resp, data := env.doJSON(http.MethodGet, "/api/routing", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d body: %v", resp.StatusCode, data)
	}
	routing, ok := data["routing"].(map[string]any)
	if !ok {
		t.Fatalf("missing routing: %v", data)
	}
	if routing["default_guest_action"] != "proxy" {
		t.Fatalf("default_guest_action: %v", routing["default_guest_action"])
	}
	if _, ok := data["preview"].([]any); !ok {
		t.Fatal("missing preview array")
	}
}

func TestAPIRoutingPUTSave(t *testing.T) {
	env := newRoutingTestEnv(t)
	defer env.close()

	resp, data := env.doJSON(http.MethodPut, "/api/routing", map[string]any{
		"routing": sampleRoutingPayload(),
		"apply":   false,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d body: %v", resp.StatusCode, data)
	}
	if data["ok"] != true {
		t.Fatalf("expected ok: %v", data)
	}

	got := env.store.Get().Routing
	if len(got.Rules) != 2 || got.Rules[0].Domains[0] != "geosite:cn" {
		t.Fatalf("store rules: %+v", got.Rules)
	}
	if got.RuleOrder[1] != "block" {
		t.Fatalf("rule_order: %v", got.RuleOrder)
	}
}

func TestAPIRoutingPUTValidation(t *testing.T) {
	env := newRoutingTestEnv(t)
	defer env.close()

	bad := sampleRoutingPayload()
	bad["domain_strategy"] = "bad-strategy"
	resp, data := env.doJSON(http.MethodPut, "/api/routing", map[string]any{
		"routing": bad,
		"apply":   false,
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %v", resp.StatusCode, data)
	}
}

func TestAPIRoutingPUTUnauthorized(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "panel.json")
	store, err := config.NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	panel := service.NewPanelService(store)
	upd := update.NewService(dir, path, store.Get)
	ts := httptest.NewServer(New(panel, upd).Router())
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/routing", bytes.NewReader([]byte(`{"routing":{}}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAPISettingsPreservesRouting(t *testing.T) {
	env := newRoutingTestEnv(t)
	defer env.close()

	_, saveData := env.doJSON(http.MethodPut, "/api/routing", map[string]any{
		"routing": sampleRoutingPayload(),
		"apply":   false,
	})
	if saveData["ok"] != true {
		t.Fatalf("routing save failed: %v", saveData)
	}

	cfg := env.store.Get()
	settingsBody := map[string]any{
		"paths": map[string]any{
			"xray_bin":        cfg.Paths.XrayBin,
			"xray_config":     cfg.Paths.XrayConfig,
			"startup_script":  cfg.Paths.StartupScript,
			"iptables_script": cfg.Paths.IptablesScript,
			"panel_data_dir":  cfg.Paths.PanelDataDir,
		},
		"network": map[string]any{
			"guest_subnet":  cfg.Network.GuestSubnet,
			"listen_addr":   cfg.Network.ListenAddr,
			"xray_api_addr": cfg.Network.XrayAPIAddr,
		},
		"iptables": map[string]any{
			"guest_subnet": cfg.Iptables.GuestSubnet,
			"tcp_port":     cfg.Iptables.TCPPort,
			"udp_port":     cfg.Iptables.UDPPort,
			"socks_port":   cfg.Iptables.SOCKSPort,
			"api_port":     cfg.Iptables.APIPort,
		},
		"observatory": map[string]any{
			"enabled":        cfg.Observatory.Enabled,
			"probe_url":      cfg.Observatory.ProbeURL,
			"probe_interval": cfg.Observatory.ProbeInterval,
		},
		"watchdog": map[string]any{
			"enabled":                          cfg.Watchdog.Enabled,
			"interval_sec":                     cfg.Watchdog.IntervalSec,
			"refresh_subscriptions_on_outage": cfg.Watchdog.RefreshSubscriptionsOnOutage,
		},
		"fail_open": map[string]any{
			"enabled":             cfg.FailOpen.EnabledOrDefault(),
			"restore_on_recovery": cfg.FailOpen.RestoreOnRecoveryOrDefault(),
			"marker_path":         cfg.FailOpen.MarkerPathOrDefault(),
		},
		"subscriptions_policy": map[string]any{
			"auto_refresh_enabled":      cfg.SubscriptionsPolicy.AutoRefreshEnabled,
			"auto_refresh_interval_min": cfg.SubscriptionsPolicy.AutoRefreshIntervalMin,
			"reselect_strategy":         cfg.SubscriptionsPolicy.ReselectStrategy,
			"auto_apply_on_change":      cfg.SubscriptionsPolicy.AutoApplyOnChange,
		},
		"logs": map[string]any{
			"startup_log":      cfg.Logs.Startup,
			"panel_log":          cfg.Logs.Panel,
			"xray_access_log":    cfg.Logs.XrayAccess,
			"xray_error_log":     cfg.Logs.XrayError,
		},
		"routing": sampleRoutingPayload(),
	}
	resp, data := env.doJSON(http.MethodPut, "/api/settings", settingsBody)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("settings save: %d %v", resp.StatusCode, data)
	}

	got := env.store.Get().Routing
	if len(got.Rules) != 2 || got.Rules[1].Action != "block" {
		t.Fatalf("routing not preserved: %+v", got.Rules)
	}
}

func TestAPISettingsIgnoresEmptyRoutingPayload(t *testing.T) {
	env := newRoutingTestEnv(t)
	defer env.close()

	_, saveData := env.doJSON(http.MethodPut, "/api/routing", map[string]any{
		"routing": sampleRoutingPayload(),
		"apply":   false,
	})
	if saveData["ok"] != true {
		t.Fatalf("routing save failed: %v", saveData)
	}

	cfg := env.store.Get()
	settingsBody := map[string]any{
		"paths": map[string]any{
			"xray_bin": cfg.Paths.XrayBin, "xray_config": cfg.Paths.XrayConfig,
			"startup_script": cfg.Paths.StartupScript, "iptables_script": cfg.Paths.IptablesScript,
			"panel_data_dir": cfg.Paths.PanelDataDir,
		},
		"network": map[string]any{
			"guest_subnet": cfg.Network.GuestSubnet, "listen_addr": cfg.Network.ListenAddr,
			"xray_api_addr": cfg.Network.XrayAPIAddr,
		},
		"iptables": map[string]any{
			"guest_subnet": cfg.Iptables.GuestSubnet, "tcp_port": cfg.Iptables.TCPPort,
			"udp_port": cfg.Iptables.UDPPort, "socks_port": cfg.Iptables.SOCKSPort, "api_port": cfg.Iptables.APIPort,
		},
		"observatory": map[string]any{
			"enabled": cfg.Observatory.Enabled, "probe_url": cfg.Observatory.ProbeURL,
			"probe_interval": cfg.Observatory.ProbeInterval,
		},
		"watchdog": map[string]any{
			"enabled": cfg.Watchdog.Enabled, "interval_sec": cfg.Watchdog.IntervalSec,
			"refresh_subscriptions_on_outage": cfg.Watchdog.RefreshSubscriptionsOnOutage,
		},
		"fail_open": map[string]any{
			"enabled": cfg.FailOpen.EnabledOrDefault(), "restore_on_recovery": cfg.FailOpen.RestoreOnRecoveryOrDefault(),
			"marker_path": cfg.FailOpen.MarkerPathOrDefault(),
		},
		"subscriptions_policy": map[string]any{
			"auto_refresh_enabled": cfg.SubscriptionsPolicy.AutoRefreshEnabled,
			"auto_refresh_interval_min": cfg.SubscriptionsPolicy.AutoRefreshIntervalMin,
			"reselect_strategy": cfg.SubscriptionsPolicy.ReselectStrategy,
			"auto_apply_on_change": cfg.SubscriptionsPolicy.AutoApplyOnChange,
		},
		"logs": map[string]any{
			"startup_log": cfg.Logs.Startup, "panel_log": cfg.Logs.Panel,
			"xray_access_log": cfg.Logs.XrayAccess, "xray_error_log": cfg.Logs.XrayError,
		},
		"routing": map[string]any{},
	}
	resp, data := env.doJSON(http.MethodPut, "/api/settings", settingsBody)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("settings save: %d %v", resp.StatusCode, data)
	}

	got := env.store.Get().Routing
	if len(got.Rules) != 2 || got.Rules[0].Domains[0] != "geosite:cn" {
		t.Fatalf("empty routing in settings must not reset rules: %+v", got.Rules)
	}
}

func TestAPISettingsUpdatesRouting(t *testing.T) {
	env := newRoutingTestEnv(t)
	defer env.close()

	cfg := env.store.Get()
	updated := sampleRoutingPayload()
	updated["default_guest_action"] = "direct"
	updated["rules"] = []map[string]any{
		{"id": "ru", "name": "RU IP", "action": "direct", "ips": []string{"geoip:ru"}, "enabled": true},
	}

	settingsBody := map[string]any{
		"paths": map[string]any{
			"xray_bin": cfg.Paths.XrayBin, "xray_config": cfg.Paths.XrayConfig,
			"startup_script": cfg.Paths.StartupScript, "iptables_script": cfg.Paths.IptablesScript,
			"panel_data_dir": cfg.Paths.PanelDataDir,
		},
		"network": map[string]any{
			"guest_subnet": cfg.Network.GuestSubnet, "listen_addr": cfg.Network.ListenAddr,
			"xray_api_addr": cfg.Network.XrayAPIAddr,
		},
		"iptables": map[string]any{
			"guest_subnet": cfg.Iptables.GuestSubnet, "tcp_port": cfg.Iptables.TCPPort,
			"udp_port": cfg.Iptables.UDPPort, "socks_port": cfg.Iptables.SOCKSPort, "api_port": cfg.Iptables.APIPort,
		},
		"observatory": map[string]any{
			"enabled": cfg.Observatory.Enabled, "probe_url": cfg.Observatory.ProbeURL,
			"probe_interval": cfg.Observatory.ProbeInterval,
		},
		"watchdog": map[string]any{
			"enabled": cfg.Watchdog.Enabled, "interval_sec": cfg.Watchdog.IntervalSec,
			"refresh_subscriptions_on_outage": cfg.Watchdog.RefreshSubscriptionsOnOutage,
		},
		"fail_open": map[string]any{
			"enabled": cfg.FailOpen.EnabledOrDefault(), "restore_on_recovery": cfg.FailOpen.RestoreOnRecoveryOrDefault(),
			"marker_path": cfg.FailOpen.MarkerPathOrDefault(),
		},
		"subscriptions_policy": map[string]any{
			"auto_refresh_enabled": cfg.SubscriptionsPolicy.AutoRefreshEnabled,
			"auto_refresh_interval_min": cfg.SubscriptionsPolicy.AutoRefreshIntervalMin,
			"reselect_strategy": cfg.SubscriptionsPolicy.ReselectStrategy,
			"auto_apply_on_change": cfg.SubscriptionsPolicy.AutoApplyOnChange,
		},
		"logs": map[string]any{
			"startup_log": cfg.Logs.Startup, "panel_log": cfg.Logs.Panel,
			"xray_access_log": cfg.Logs.XrayAccess, "xray_error_log": cfg.Logs.XrayError,
		},
		"routing": updated,
	}
	resp, _ := env.doJSON(http.MethodPut, "/api/settings", settingsBody)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("settings save: %d", resp.StatusCode)
	}
	if env.store.Get().Routing.DefaultGuest != "direct" {
		t.Fatalf("routing not updated via settings")
	}
}
