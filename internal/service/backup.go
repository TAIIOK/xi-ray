package service

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

type BackupPayload struct {
	ExportedAt time.Time          `json:"exported_at"`
	Version    int                `json:"version"`
	Config     config.PanelConfig `json:"config"`
}

func (p *PanelService) ExportBackup() ([]byte, error) {
	cfg := p.store.Get()
	payload := BackupPayload{
		ExportedAt: time.Now(),
		Version:    config.CurrentVersion,
		Config:     cfg,
	}
	return json.MarshalIndent(payload, "", "  ")
}

func (p *PanelService) RestoreBackup(data []byte) error {
	var payload BackupPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("invalid backup json: %w", err)
	}
	cfg := payload.Config
	if cfg.Version == 0 && payload.Version > 0 {
		cfg.Version = payload.Version
	}
	cfg.Normalize()
	if err := config.ValidatePaths(cfg.Paths); err != nil {
		return err
	}
	if cfg.Auth.Username == "" || cfg.Auth.PasswordHash == "" {
		return fmt.Errorf("backup missing auth credentials")
	}

	currentPath := p.store.Path()
	p.store.Replace(cfg)
	if err := p.store.Save(); err != nil {
		return err
	}
	_ = currentPath // config path unchanged
	return nil
}

func (p *PanelService) RestoreBackupFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return p.RestoreBackup(data)
}
