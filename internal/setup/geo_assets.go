package setup

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func XrayBinDir(xrayBin string) string {
	return filepath.Dir(xrayBin)
}

func geoAssetDestinations(xrayBin string) (binDir, geoip, geosite string) {
	binDir = XrayBinDir(xrayBin)
	return binDir, filepath.Join(binDir, "geoip.dat"), filepath.Join(binDir, "geosite.dat")
}

func legacyGeoPaths(xrayBin string) (geoip, geosite string) {
	parent := filepath.Dir(XrayBinDir(xrayBin))
	return filepath.Join(parent, "geoip.dat"), filepath.Join(parent, "geosite.dat")
}

func geoAssetsPresent(geoip, geosite string) bool {
	return fileExists(geoip) && fileExists(geosite)
}

// EnsureGeoAssets makes geoip.dat and geosite.dat available next to the Xray binary.
// Xray 26 resolves geo files from the binary directory unless XRAY_LOCATION_ASSET is set.
func EnsureGeoAssets(ctx context.Context, xrayBin string) error {
	if xrayBin == "" {
		return fmt.Errorf("путь к Xray не задан")
	}
	if err := migrateGeoAssetsToBin(xrayBin); err != nil {
		return err
	}
	_, geoip, geosite := geoAssetDestinations(xrayBin)
	if geoAssetsPresent(geoip, geosite) {
		return nil
	}

	client := &http.Client{Timeout: 10 * time.Minute}
	for _, item := range []struct {
		url, dest, label string
	}{
		{geoipDatURL, geoip, "geoip.dat"},
		{geositeDatURL, geosite, "geosite.dat"},
	} {
		if fileExists(item.dest) {
			continue
		}
		if err := downloadFile(ctx, client, item.url, item.dest, 0o644); err != nil {
			return fmt.Errorf("download %s: %w", item.label, err)
		}
	}
	if !geoAssetsPresent(geoip, geosite) {
		binDir := XrayBinDir(xrayBin)
		return fmt.Errorf("geoip.dat и geosite.dat нужны в %s — нажмите «Скачать Xray»", binDir)
	}
	return nil
}

func migrateGeoAssetsToBin(xrayBin string) error {
	binDir, geoip, geosite := geoAssetDestinations(xrayBin)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("geo assets dir: %w", err)
	}
	if geoAssetsPresent(geoip, geosite) {
		return nil
	}
	legGeo, legSite := legacyGeoPaths(xrayBin)
	if fileExists(legGeo) && !fileExists(geoip) {
		if err := copyFile(legGeo, geoip, 0o644); err != nil {
			return fmt.Errorf("copy geoip.dat to bin: %w", err)
		}
	}
	if fileExists(legSite) && !fileExists(geosite) {
		if err := copyFile(legSite, geosite, 0o644); err != nil {
			return fmt.Errorf("copy geosite.dat to bin: %w", err)
		}
	}
	return nil
}

func copyFile(src, dest string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".geo-copy-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		return err
	}
	return os.Chmod(dest, mode)
}
