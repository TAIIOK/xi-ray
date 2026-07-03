package update

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

type ReleaseInfo struct {
	Version     string `json:"version"`
	Tag         string `json:"tag"`
	DownloadURL string `json:"download_url"`
	NotesURL    string `json:"notes_url"`
	Body        string `json:"body,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
}

type githubRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	} `json:"assets"`
}

func Repo() string {
	if v := strings.TrimSpace(os.Getenv("PANEL_UPDATE_REPO")); v != "" {
		return v
	}
	return repoDefault
}

func FetchLatestRelease(client *http.Client) (ReleaseInfo, error) {
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", Repo())
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return ReleaseInfo{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "xiaomi-vless-panel/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return ReleaseInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return ReleaseInfo{}, fmt.Errorf("github api: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return ReleaseInfo{}, err
	}
	tag := strings.TrimSpace(rel.TagName)
	if tag == "" {
		return ReleaseInfo{}, fmt.Errorf("release missing tag")
	}

	info := ReleaseInfo{
		Version:     strings.TrimPrefix(tag, "v"),
		Tag:         tag,
		NotesURL:    fmt.Sprintf("https://github.com/%s/releases/tag/%s", Repo(), tag),
		Body:        rel.Body,
		PublishedAt: rel.PublishedAt.UTC().Format(time.RFC3339),
	}

	prefix := fmt.Sprintf("xiaomi-vless-%s-linux-arm64.tar.gz", tag)
	for _, asset := range rel.Assets {
		if asset.Name == prefix {
			info.DownloadURL = asset.BrowserDownloadURL
			break
		}
	}
	if info.DownloadURL == "" {
		names := make([]string, 0, len(rel.Assets))
		for _, a := range rel.Assets {
			names = append(names, a.Name)
		}
		sort.Strings(names)
		return ReleaseInfo{}, fmt.Errorf("release %s missing bundle asset %q (have: %s)", tag, prefix, strings.Join(names, ", "))
	}
	return info, nil
}

func verifyFileSHA256(path, want string) error {
	want = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(want)), "sha256:")
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if want != "" && got != want {
		return fmt.Errorf("checksum mismatch: want %s got %s", want, got)
	}
	return nil
}

func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
