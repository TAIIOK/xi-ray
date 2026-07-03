package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxDownloadAttempts = 3

func DownloadArchive(ctx context.Context, client *http.Client, url, dest string, st State, onProgress func(State)) (State, error) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Minute}
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return st, err
	}

	var offset int64
	if info, err := os.Stat(dest); err == nil {
		offset = info.Size()
		st.DownloadedBytes = offset
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return st, err
	}
	req.Header.Set("User-Agent", "xiaomi-vless-panel/1.0")
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	resp, err := client.Do(req)
	if err != nil {
		return st, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		offset = 0
		st.DownloadedBytes = 0
	case http.StatusPartialContent:
		// resume
	case http.StatusRequestedRangeNotSatisfiable:
		offset = 0
		st.DownloadedBytes = 0
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return st, fmt.Errorf("download HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if cl := resp.ContentLength; cl > 0 {
		if resp.StatusCode == http.StatusPartialContent {
			st.TotalBytes = offset + cl
		} else {
			st.TotalBytes = cl
		}
	} else if cr := resp.Header.Get("Content-Range"); cr != "" {
		if total, ok := parseContentRangeTotal(cr); ok {
			st.TotalBytes = total
		}
	}

	flags := os.O_CREATE | os.O_WRONLY
	if offset == 0 {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(dest, flags, 0o644)
	if err != nil {
		return st, err
	}
	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			f.Close()
			return st, err
		}
	}

	written, err := io.Copy(f, resp.Body)
	f.Close()
	if err != nil {
		return st, err
	}
	st.DownloadedBytes = offset + written
	if onProgress != nil {
		onProgress(st)
	}
	if st.TotalBytes > 0 && st.DownloadedBytes < st.TotalBytes {
		return st, fmt.Errorf("incomplete download: %d/%d bytes", st.DownloadedBytes, st.TotalBytes)
	}
	return st, nil
}

func parseContentRangeTotal(v string) (int64, bool) {
	// bytes 0-1023/2048
	parts := strings.Split(v, "/")
	if len(parts) != 2 {
		return 0, false
	}
	var total int64
	if _, err := fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &total); err != nil {
		return 0, false
	}
	return total, true
}

func VerifyArchiveChecksum(path, want string) error {
	return verifyFileSHA256(path, want)
}

func ExtractArchive(archivePath, stagingDir string) error {
	if err := os.RemoveAll(stagingDir); err != nil {
		return err
	}
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		return err
	}

	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target := filepath.Join(stagingDir, hdr.Name)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(stagingDir)+string(os.PathSeparator)) && filepath.Clean(target) != filepath.Clean(stagingDir) {
			return fmt.Errorf("unsafe path in archive: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Sync(); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
	return nil
}

func CleanupPartialDownload(path string) {
	_ = os.Remove(path)
}
