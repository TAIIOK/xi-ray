package update

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadArchiveResume(t *testing.T) {
	body := []byte("0123456789abcdef")
	var served int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "" {
			w.WriteHeader(http.StatusPartialContent)
			fmt.Fprint(w, string(body[8:]))
			served++
			return
		}
		w.Write(body[:8])
		served++
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "file.bin")
	if err := os.WriteFile(dest, body[:8], 0o644); err != nil {
		t.Fatal(err)
	}

	st := State{DownloadedBytes: 8}
	st, err := DownloadArchive(context.Background(), srv.Client(), srv.URL, dest, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	if st.DownloadedBytes != int64(len(body)) {
		t.Fatalf("bytes=%d", st.DownloadedBytes)
	}
	data, _ := os.ReadFile(dest)
	if string(data) != string(body) {
		t.Fatalf("content mismatch")
	}
}

func TestVerifyFileSHA256(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f")
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum, err := FileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyFileSHA256(path, sum); err != nil {
		t.Fatal(err)
	}
	if err := verifyFileSHA256(path, "bad"); err == nil {
		t.Fatal("expected mismatch")
	}
}
