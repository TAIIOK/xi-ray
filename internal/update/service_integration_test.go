package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

func TestStatusCanDownloadNotDuringDownloading(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "panel.json")
	if err := os.WriteFile(cfgPath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := NewService(dir, cfgPath, func() config.PanelConfig { return config.PanelConfig{} })
	store := NewStateStore(filepath.Join(dir, "updates", "state.json"))
	if err := store.Save(State{Phase: PhaseDownloading, TargetVersion: "9.9.9"}); err != nil {
		t.Fatal(err)
	}
	st, err := svc.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.CanDownload {
		t.Fatal("CanDownload should be false during downloading")
	}
}

func TestResumeExtractingCompletesToVerified(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "panel.json")
	if err := os.WriteFile(cfgPath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := NewService(dir, cfgPath, func() config.PanelConfig { return config.PanelConfig{} })
	layout := svc.Layout()
	if err := layout.EnsureDirs(); err != nil {
		t.Fatal(err)
	}

	panelData := []byte("panel-binary")
	archivePath := filepath.Join(layout.DownloadsDir, "v9.9.9.tar.gz")
	if err := writeTestBundle(archivePath, panelData, "9.9.9"); err != nil {
		t.Fatal(err)
	}

	store := NewStateStore(layout.StatePath)
	if err := store.Save(State{
		Phase:         PhaseExtracting,
		TargetVersion: "9.9.9",
		ArchivePath:   archivePath,
	}); err != nil {
		t.Fatal(err)
	}

	if err := svc.resumeExtracting(context.Background()); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Phase != PhaseVerified {
		t.Fatalf("phase = %q, want verified", loaded.Phase)
	}
	if _, err := os.Stat(filepath.Join(layout.StagingDir, "panel")); err != nil {
		t.Fatal(err)
	}
}

func TestServiceDownloadToVerified(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "panel.json")
	if err := os.WriteFile(cfgPath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	panelData := []byte("integration-panel")
	bundle, err := buildIntegrationTestBundle(panelData, "9.9.9")
	if err != nil {
		t.Fatal(err)
	}

	var mock *httptest.Server
	mock = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/TAIIOK/xi-ray/releases/latest":
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
		case r.URL.Path == "/bundle.tar.gz":
			w.Header().Set("Content-Length", fmt.Sprint(len(bundle)))
			_, _ = w.Write(bundle)
		default:
			http.NotFound(w, r)
		}
	}))
	defer mock.Close()

	t.Setenv("PANEL_UPDATE_REPO", "TAIIOK/xi-ray")
	client := CheckHTTPClient()
	client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		isGitHub := req.URL.Host == "api.github.com" || strings.Contains(req.URL.Path, "releases/latest")
		req.URL.Scheme = "http"
		req.URL.Host = mock.Listener.Addr().String()
		if isGitHub {
			req.URL.Path = "/repos/TAIIOK/xi-ray/releases/latest"
		} else {
			req.URL.Path = "/bundle.tar.gz"
		}
		return http.DefaultTransport.RoundTrip(req)
	})

	svc := NewService(dir, cfgPath, func() config.PanelConfig { return config.PanelConfig{} })
	svc.checkClient = client
	svc.downloadClient = client

	if _, err := svc.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	st, err := svc.Download(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if st.Phase != PhaseVerified {
		t.Fatalf("phase = %q, want verified (error=%q)", st.Phase, st.Error)
	}
	if st.TargetVersion != "9.9.9" {
		t.Fatalf("target version = %q", st.TargetVersion)
	}
}

func TestManifestValidatePlatformMismatch(t *testing.T) {
	m := Manifest{Platform: "linux/invalid-arch"}
	if err := m.ValidatePlatform(); err == nil {
		t.Fatal("expected platform mismatch error")
	}
}

func TestManifestValidatePlatformMatch(t *testing.T) {
	m := Manifest{Platform: runtime.GOOS + "/" + runtime.GOARCH}
	if err := m.ValidatePlatform(); err != nil {
		t.Fatal(err)
	}
}

func buildIntegrationTestBundle(panelData []byte, version string) ([]byte, error) {
	sum, err := FileSHA256FromBytes(panelData)
	if err != nil {
		return nil, err
	}
	manifest := Manifest{
		Version:          version,
		MinConfigVersion: 1,
		Platform:         runtime.GOOS + "/" + runtime.GOARCH,
		Assets: map[string]ManifestAsset{
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

func writeTestBundle(path string, panelData []byte, version string) error {
	bundle, err := buildIntegrationTestBundle(panelData, version)
	if err != nil {
		return err
	}
	return os.WriteFile(path, bundle, 0o644)
}

func FileSHA256FromBytes(data []byte) (string, error) {
	tmp := filepath.Join(os.TempDir(), "sha-test")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", err
	}
	defer os.Remove(tmp)
	return FileSHA256(tmp)
}
