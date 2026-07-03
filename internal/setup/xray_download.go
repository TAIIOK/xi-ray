package setup

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

const (
	xrayZipURL    = "https://github.com/XTLS/Xray-core/releases/latest/download/Xray-linux-arm64-v8a.zip"
	geoipDatURL   = "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat"
	geositeDatURL = "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat"
)

type XrayDownloadResult struct {
	OK      bool          `json:"ok"`
	Message string        `json:"message"`
	Version string        `json:"version,omitempty"`
	Paths   config.Paths  `json:"paths"`
	Actions []SetupAction `json:"actions"`
	Checks  []PathCheck   `json:"checks"`
}

type XrayDownloadOptions struct {
	USBMount string
	XrayBin  string
}

func ResolveInstallBase(opts XrayDownloadOptions) (string, error) {
	if mount := strings.TrimRight(strings.TrimSpace(opts.USBMount), "/"); mount != "" {
		if err := validateInstallBase(mount); err != nil {
			return "", err
		}
		return mount, nil
	}
	if bin := strings.TrimSpace(opts.XrayBin); bin != "" {
		base := usbPathFromXrayBin(bin)
		if err := validateInstallBase(base); err != nil {
			return "", err
		}
		return base, nil
	}
	for _, m := range DiscoverUSBMounts() {
		if pathWritable(m.Path) {
			return m.Path, nil
		}
	}
	return "", fmt.Errorf("выберите USB-раздел или укажите путь к Xray binary")
}

func validateInstallBase(base string) error {
	base = filepath.Clean(base)
	if base == "" || base == "/" || base == "." {
		return fmt.Errorf("некорректный каталог установки: %s", base)
	}
	if strings.Contains(base, "..") {
		return fmt.Errorf("некорректный путь: %s", base)
	}
	if strings.HasPrefix(base, "/mnt/") {
		if !pathWritable(base) && !pathWritable(filepath.Dir(base)) {
			return fmt.Errorf("каталог недоступен для записи: %s", base)
		}
		return nil
	}
	if pathWritable(base) {
		return nil
	}
	return fmt.Errorf("установка Xray только на USB (/mnt/…) — получено: %s", base)
}

func DownloadXray(ctx context.Context, base string) XrayDownloadResult {
	result := XrayDownloadResult{
		Paths: PathsFromUSBMount(base),
	}
	add := func(action, path, msg string, ok bool) {
		result.Actions = append(result.Actions, SetupAction{Action: action, Path: path, OK: ok, Message: msg})
	}

	if err := validateInstallBase(base); err != nil {
		result.Message = err.Error()
		add("validate", base, err.Error(), false)
		return result
	}

	xrayDir := filepath.Join(base, "xray")
	binDir := filepath.Join(xrayDir, "bin")
	xrayBin := filepath.Join(binDir, "xray")
	configPath := filepath.Join(xrayDir, "config.json")

	for _, dir := range []string{binDir, xrayDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			result.Message = err.Error()
			add("mkdir", dir, err.Error(), false)
			return result
		}
		add("mkdir", dir, "ready", true)
	}

	client := &http.Client{Timeout: 10 * time.Minute}

	if err := downloadAndExtractXray(ctx, client, xrayBin); err != nil {
		result.Message = err.Error()
		add("download", xrayZipURL, err.Error(), false)
		result.Checks = VerifyPaths(result.Paths)
		return result
	}
	add("download", xrayBin, "Xray binary installed", true)

	for _, item := range []struct {
		url, dest, label string
	}{
		{geoipDatURL, filepath.Join(binDir, "geoip.dat"), "geoip.dat"},
		{geositeDatURL, filepath.Join(binDir, "geosite.dat"), "geosite.dat"},
	} {
		if err := downloadFile(ctx, client, item.url, item.dest, 0o644); err != nil {
			result.Message = err.Error()
			add("download", item.dest, err.Error(), false)
			result.Checks = VerifyPaths(result.Paths)
			return result
		}
		add("download", item.dest, item.label+" installed", true)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := os.WriteFile(configPath, []byte("{}\n"), 0o644); err != nil {
			add("create", configPath, err.Error(), false)
		} else {
			add("create", configPath, "placeholder config (Apply перезапишет)", true)
		}
	} else {
		add("skip", configPath, "config already exists", true)
	}

	result.Version = XrayVersion(xrayBin)
	result.Checks = VerifyPaths(result.Paths)
	result.OK = AllChecksOK(result.Checks)
	if result.OK {
		result.Message = fmt.Sprintf("Xray установлен в %s", xrayDir)
		if result.Version != "" {
			result.Message += " (" + result.Version + ")"
		}
	} else {
		result.Message = "Xray скачан, но не все проверки пройдены — нажмите «Установить недостающее»"
	}
	return result
}

func downloadAndExtractXray(ctx context.Context, client *http.Client, destBin string) error {
	tmpZip, err := os.CreateTemp(filepath.Dir(destBin), "xray-*.zip")
	if err != nil {
		return err
	}
	tmpPath := tmpZip.Name()
	defer os.Remove(tmpPath)

	if err := downloadFile(ctx, client, xrayZipURL, tmpPath, 0o600); err != nil {
		tmpZip.Close()
		return err
	}
	if err := tmpZip.Close(); err != nil {
		return err
	}

	if err := extractXrayBundle(tmpPath, destBin); err != nil {
		return err
	}
	return os.Chmod(destBin, 0o755)
}

func extractXrayBundle(zipPath, destBin string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	binDir := filepath.Dir(destBin)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}

	var xrayFile *zip.File
	for _, f := range r.File {
		name := filepath.Base(f.Name)
		if f.FileInfo().IsDir() {
			continue
		}
		switch name {
		case "xray":
			xrayFile = f
		case "geoip.dat", "geosite.dat":
			if err := extractZipEntry(f, filepath.Join(binDir, name), 0o644); err != nil {
				return err
			}
		}
	}
	if xrayFile == nil {
		return fmt.Errorf("xray binary not found in zip")
	}
	return extractZipEntry(xrayFile, destBin, 0o755)
}

func extractZipEntry(f *zip.File, dest string, mode os.FileMode) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, rc); err != nil {
		return err
	}
	if err := out.Sync(); err != nil {
		return err
	}
	return os.Chmod(dest, mode)
}

func downloadFile(ctx context.Context, client *http.Client, url, dest string, mode os.FileMode) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "xiaomi-vless-panel/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".download-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	written, err := io.Copy(tmp, resp.Body)
	tmp.Close()
	if err != nil {
		os.Remove(tmpPath)
		return err
	}
	if written == 0 {
		os.Remove(tmpPath)
		return fmt.Errorf("empty download from %s", url)
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if mode != 0 {
		return os.Chmod(dest, mode)
	}
	return nil
}
