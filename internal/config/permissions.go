package config

import "os"

const (
	PanelDirPerm   = 0o755 // xray and other tools must read configs inside
	ConfigFilePerm = 0o644
	SecretFilePerm = 0o600
)

// EnsureDir creates dir (and parents) with perm, fixing mode if dir already exists too restrictive.
func EnsureDir(path string, perm os.FileMode) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(path, perm); err != nil {
		return err
	}
	return os.Chmod(path, perm)
}
