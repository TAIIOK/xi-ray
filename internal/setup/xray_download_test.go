package setup

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveInstallBase(t *testing.T) {
	dir := t.TempDir()
	mount := filepath.Join(dir, "usb-test")
	if err := os.MkdirAll(mount, 0o755); err != nil {
		t.Fatal(err)
	}

	base, err := ResolveInstallBase(XrayDownloadOptions{USBMount: mount})
	if err != nil {
		t.Fatal(err)
	}
	if base != mount {
		t.Fatalf("got %s", base)
	}

	xrayBin := filepath.Join(mount, "xray", "bin", "xray")
	base, err = ResolveInstallBase(XrayDownloadOptions{XrayBin: xrayBin})
	if err != nil {
		t.Fatal(err)
	}
	if base != mount {
		t.Fatalf("got %s", base)
	}
}

func TestValidateInstallBaseRejectsRoot(t *testing.T) {
	if err := validateInstallBase("/"); err == nil {
		t.Fatal("expected error for /")
	}
}

func TestExtractXrayBundle(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "xray.zip")
	dest := filepath.Join(dir, "bin", "xray")

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, item := range []struct {
		name, body string
	}{
		{"xray", "#!/bin/sh\necho xray-test\n"},
		{"geoip.dat", "geo-data"},
		{"geosite.dat", "site-data"},
	} {
		w, err := zw.Create(item.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(item.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(zipPath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := extractXrayBundle(zipPath, dest); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatal("expected executable")
	}
	for _, name := range []string{"geoip.dat", "geosite.dat"} {
		if _, err := os.Stat(filepath.Join(dir, "bin", name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
}
