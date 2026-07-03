package xray

import (
	"path/filepath"
	"strings"
)

// StagingConfigPath returns a writable staging path with a .json extension.
// Xray 26+ detects format by extension; config.json.new is rejected.
// Staging lives in panel data dir (usually /data) — not on USB — so it survives failed apply.
func StagingConfigPath(panelDataDir, finalConfigPath string) string {
	if dir := strings.TrimSpace(panelDataDir); dir != "" {
		return filepath.Join(dir, "config.staging.json")
	}
	dir := filepath.Dir(finalConfigPath)
	base := filepath.Base(finalConfigPath)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	if name == "" {
		name = "config"
	}
	return filepath.Join(dir, name+".staging.json")
}

// XrayAssetDir is the directory containing geoip.dat / geosite.dat (parent of bin/).
// Prefer XrayBinDir for the path Xray 26 uses when resolving geo files.
func XrayAssetDir(xrayBin string) string {
	return filepath.Dir(filepath.Dir(xrayBin))
}

// XrayBinDir is the directory containing the xray binary (geo files belong here).
func XrayBinDir(xrayBin string) string {
	return filepath.Dir(xrayBin)
}

// LastGeneratedConfigPath keeps the last generated config for debugging.
func LastGeneratedConfigPath(panelDataDir string) string {
	return filepath.Join(panelDataDir, "last-generated-config.json")
}
