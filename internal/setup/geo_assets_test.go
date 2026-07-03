package setup

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGeoAssetDestinations(t *testing.T) {
	bin := "/mnt/usb-abc/xray/bin/xray"
	dir, geoip, geosite := geoAssetDestinations(bin)
	if dir != "/mnt/usb-abc/xray/bin" {
		t.Fatalf("bin dir = %q", dir)
	}
	if geoip != filepath.Join(dir, "geoip.dat") {
		t.Fatalf("geoip = %q", geoip)
	}
	if geosite != filepath.Join(dir, "geosite.dat") {
		t.Fatalf("geosite = %q", geosite)
	}
}

func TestEnsureGeoAssetsMigratesLegacyLocation(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "xray", "bin")
	xrayDir := filepath.Join(root, "xray")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	xrayBin := filepath.Join(binDir, "xray")
	if err := os.WriteFile(xrayBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(xrayDir, "geoip.dat"), []byte("geo"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(xrayDir, "geosite.dat"), []byte("site"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureGeoAssets(context.Background(), xrayBin); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"geoip.dat", "geosite.dat"} {
		path := filepath.Join(binDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s in bin: %v", name, err)
		}
	}
}
