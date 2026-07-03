package update

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStateStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	store := NewStateStore(path)

	st := State{Phase: PhaseDownloading, TargetVersion: "1.2.3", DownloadedBytes: 100, TotalBytes: 200}
	if err := store.Save(st); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Phase != PhaseDownloading || loaded.TargetVersion != "1.2.3" {
		t.Fatalf("unexpected state: %+v", loaded)
	}
}

func TestManifestValidateStaging(t *testing.T) {
	dir := t.TempDir()
	content := []byte("hello")
	path := filepath.Join(dir, "panel")
	if err := os.WriteFile(path, content, 0o755); err != nil {
		t.Fatal(err)
	}
	sum, err := FileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	m := Manifest{
		Version: "1.0.0",
		Assets: map[string]ManifestAsset{
			"panel": {Path: "panel", SHA256: sum},
		},
	}
	if err := m.ValidateStaging(dir); err != nil {
		t.Fatal(err)
	}
}

func TestPhaseHelpers(t *testing.T) {
	if PhaseIdle.IsActive() {
		t.Fatal("idle should not be active")
	}
	if !PhaseDownloading.IsActive() {
		t.Fatal("downloading should be active")
	}
	if !PhaseDownloading.NeedsResume() {
		t.Fatal("downloading should need resume")
	}
}

func TestTrimVersion(t *testing.T) {
	if trimVersion("v1.2.3") != "1.2.3" {
		t.Fatal(trimVersion("v1.2.3"))
	}
}

func TestArchivePath(t *testing.T) {
	got := ArchivePath("/tmp/dl", "v1.0.0")
	want := filepath.Join("/tmp/dl", "v1.0.0.tar.gz")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
