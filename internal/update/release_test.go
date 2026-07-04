package update

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchLatestReleaseSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/TAIIOK/xi-ray/releases/latest" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v9.9.9",
			"assets": []map[string]string{
				{"name": "xiaomi-vless-v9.9.9-linux-arm64.tar.gz", "browser_download_url": "https://example.com/bundle.tar.gz"},
			},
		})
	}))
	defer srv.Close()

	t.Setenv("PANEL_UPDATE_REPO", "TAIIOK/xi-ray")
	client := CheckHTTPClient()
	client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = "http"
		req.URL.Host = srv.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(req)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info, err := FetchLatestRelease(ctx, client)
	if err != nil {
		t.Fatal(err)
	}
	if info.Version != "9.9.9" {
		t.Fatalf("version = %q", info.Version)
	}
	if info.DownloadURL == "" {
		t.Fatal("missing download url")
	}
}

func TestFetchLatestReleaseTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := CheckHTTPClient()
	client.Timeout = 100 * time.Millisecond
	client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = "http"
		req.URL.Host = srv.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(req)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := FetchLatestRelease(ctx, client)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
