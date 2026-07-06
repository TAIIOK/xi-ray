package server

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/taiiok/xiaomi-vless/internal/config"
	"github.com/taiiok/xiaomi-vless/internal/service"
	"github.com/taiiok/xiaomi-vless/internal/update"
)

func newUpdateTestEnv(t *testing.T, mock *httptest.Server, bundle []byte) (*testEnv, *update.Service) {
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
	if err := store.Update(func(cfg *config.PanelConfig) error {
		cfg.Setup.OnboardingCompleted = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	panel := service.NewPanelService(store)
	upd := update.NewService(dir, path, store.Get)
	client := update.CheckHTTPClient()
	client.Transport = updateRoundTripper{mock: mock}
	upd.SetHTTPClients(client, client)

	srv := New(panel, upd)
	ts := httptest.NewServer(srv.Router())

	jar := &cookieJar{}
	httpClient := &http.Client{
		Jar:           jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	loginReq, err := http.NewRequest(http.MethodPost, ts.URL+"/login", strings.NewReader("username=admin&password=secret"))
	if err != nil {
		t.Fatal(err)
	}
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginResp, err := httpClient.Do(loginReq)
	if err != nil {
		t.Fatal(err)
	}
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login status: %d", loginResp.StatusCode)
	}

	return &testEnv{server: ts, store: store, client: httpClient}, upd
}

type updateRoundTripper struct {
	mock *httptest.Server
}

func (u updateRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	isGitHub := req.URL.Host == "api.github.com" || strings.Contains(req.URL.Path, "releases/latest")
	req.URL.Scheme = "http"
	req.URL.Host = u.mock.Listener.Addr().String()
	if isGitHub {
		req.URL.Path = "/repos/TAIIOK/xi-ray/releases/latest"
	} else {
		req.URL.Path = "/bundle.tar.gz"
	}
	return http.DefaultTransport.RoundTrip(req)
}

func TestAPIUpdateCheckDownloadFlow(t *testing.T) {
	panelData := []byte("handler-test-panel")
	bundle, err := buildHandlerTestBundle(panelData, "9.9.9")
	if err != nil {
		t.Fatal(err)
	}

	var mock *httptest.Server
	mock = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/TAIIOK/xi-ray/releases/latest":
			bundleURL := "http://" + mock.Listener.Addr().String() + "/bundle.tar.gz"
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tag_name": "v9.9.9",
				"assets": []map[string]string{
					{
						"name":                 "xiaomi-vless-v9.9.9-linux-arm64.tar.gz",
						"browser_download_url": bundleURL,
					},
				},
			})
		case "/bundle.tar.gz":
			_, _ = w.Write(bundle)
		default:
			http.NotFound(w, r)
		}
	}))
	defer mock.Close()

	t.Setenv("PANEL_UPDATE_REPO", "TAIIOK/xi-ray")
	env, _ := newUpdateTestEnv(t, mock, bundle)
	defer env.close()

	resp, data := env.doJSON(http.MethodGet, "/api/update/check", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("check status: %d body: %v", resp.StatusCode, data)
	}
	available, ok := data["available"].(map[string]any)
	if !ok {
		t.Fatalf("missing available: %v", data)
	}
	if available["version"] != "9.9.9" {
		t.Fatalf("available version: %v", available["version"])
	}

	resp, data = env.doJSON(http.MethodPost, "/api/update/download", map[string]any{})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("download status: %d body: %v", resp.StatusCode, data)
	}
	if data["phase"] != "verified" {
		t.Fatalf("phase after download: %v error=%v", data["phase"], data["error"])
	}
	if data["can_apply"] != true {
		t.Fatalf("can_apply: %v", data["can_apply"])
	}
}

func TestAPIUpdateApplySetsApplyingPhase(t *testing.T) {
	t.Setenv("XIAOMI_VLESS_KEEP_UPDATER_SCRIPT", "1")

	dir := t.TempDir()
	t.Cleanup(func() {
		// Detached nohup updater may still be closing panel-update.log.
		time.Sleep(200 * time.Millisecond)
		_ = os.Remove(filepath.Join(dir, "panel-update.log"))
	})

	path := filepath.Join(dir, "panel.json")
	if err := os.WriteFile(path, []byte(`{"paths":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := config.NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetPassword("admin", "secret"); err != nil {
		t.Fatal(err)
	}
	if err := store.Update(func(cfg *config.PanelConfig) error {
		cfg.Setup.OnboardingCompleted = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	panel := service.NewPanelService(store)
	upd := update.NewService(dir, path, store.Get)
	layout := upd.Layout()
	if err := layout.EnsureDirs(); err != nil {
		t.Fatal(err)
	}

	panelData := []byte("apply-test-panel")
	if err := os.WriteFile(filepath.Join(layout.StagingDir, "panel"), panelData, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := json.Marshal(update.Manifest{
		Version:          "9.9.9",
		MinConfigVersion: 1,
		Platform:         runtime.GOOS + "/" + runtime.GOARCH,
		Assets: map[string]update.ManifestAsset{
			"panel": {Path: "panel", SHA256: mustSHA256(t, panelData)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(layout.StagingDir, "manifest.json"), manifestBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	storePath := filepath.Join(dir, "updates", "state.json")
	if err := update.NewStateStore(storePath).Save(update.State{Phase: update.PhaseVerified, TargetVersion: "9.9.9"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(layout.UpdaterScript, []byte("#!/bin/sh\n# test stub\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	srv := New(panel, upd)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	jar := &cookieJar{}
	client := &http.Client{
		Jar:           jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	loginReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/login", strings.NewReader("username=admin&password=secret"))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginResp, err := client.Do(loginReq)
	if err != nil {
		t.Fatal(err)
	}
	loginResp.Body.Close()

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/update/apply", bytes.NewReader([]byte("{}")))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("apply status: %d body: %v", resp.StatusCode, data)
	}
	if data["phase"] != "applying" {
		t.Fatalf("phase after apply: %v", data["phase"])
	}
}

func buildHandlerTestBundle(panelData []byte, version string) ([]byte, error) {
	sum, err := sha256Hex(panelData)
	if err != nil {
		return nil, err
	}
	manifest := update.Manifest{
		Version:          version,
		MinConfigVersion: 1,
		Platform:         runtime.GOOS + "/" + runtime.GOARCH,
		Assets: map[string]update.ManifestAsset{
			"panel": {Path: "panel", SHA256: sum},
		},
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, data := range map[string][]byte{"panel": panelData, "manifest.json": manifestBytes} {
		mode := int64(0o644)
		if name == "panel" {
			mode = 0o755
		}
		hdr := &tar.Header{Name: name, Mode: mode, Size: int64(len(data))}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write(data); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func sha256Hex(data []byte) (string, error) {
	tmp := filepath.Join(os.TempDir(), "handler-sha")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", err
	}
	defer os.Remove(tmp)
	return update.FileSHA256(tmp)
}

func mustSHA256(t *testing.T, data []byte) string {
	t.Helper()
	sum, err := sha256Hex(data)
	if err != nil {
		t.Fatal(err)
	}
	return sum
}