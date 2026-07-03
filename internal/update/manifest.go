package update

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type ManifestAsset struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type Manifest struct {
	Version          string                   `json:"version"`
	MinConfigVersion int                      `json:"min_config_version"`
	Platform         string                   `json:"platform"`
	ReleasedAt       string                   `json:"released_at"`
	Assets           map[string]ManifestAsset `json:"assets"`
	NotesURL         string                   `json:"notes_url"`
}

func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest: %w", err)
	}
	if m.Version == "" {
		return Manifest{}, fmt.Errorf("manifest missing version")
	}
	if len(m.Assets) == 0 {
		return Manifest{}, fmt.Errorf("manifest missing assets")
	}
	return m, nil
}

func (m Manifest) ValidateStaging(stagingDir string) error {
	for rel, asset := range m.Assets {
		path := filepath.Join(stagingDir, asset.Path)
		if asset.Path == "" {
			path = filepath.Join(stagingDir, rel)
		}
		if err := verifyFileSHA256(path, asset.SHA256); err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
	}
	return nil
}
