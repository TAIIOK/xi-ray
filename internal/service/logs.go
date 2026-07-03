package service

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

type LogResponse struct {
	Source  string   `json:"source"`
	Path    string   `json:"path"`
	Lines   []string `json:"lines"`
	Total   int      `json:"total"`
	Message string   `json:"message,omitempty"`
}

func (p *PanelService) GetLogs(source string, maxLines int) (*LogResponse, error) {
	if maxLines <= 0 {
		maxLines = 200
	}
	if maxLines > 2000 {
		maxLines = 2000
	}

	cfg := p.store.Get()
	path, err := logPathForSource(cfg, source)
	if err != nil {
		return nil, err
	}

	lines, total, readErr := tailFile(path, maxLines)
	resp := &LogResponse{
		Source: source,
		Path:   path,
		Lines:  lines,
		Total:  total,
	}
	if readErr != nil {
		resp.Message = readErr.Error()
	}
	return resp, nil
}

func logPathForSource(cfg config.PanelConfig, source string) (string, error) {
	switch strings.ToLower(source) {
	case "startup", "":
		return cfg.Logs.Startup, nil
	case "panel":
		return cfg.Logs.Panel, nil
	case "xray-access", "access":
		return cfg.Logs.XrayAccess, nil
	case "xray-error", "error":
		return cfg.Logs.XrayError, nil
	default:
		return "", fmt.Errorf("unknown log source: %s", source)
	}
}

func tailFile(path string, maxLines int) ([]string, int, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, 0, fmt.Errorf("log file not found")
		}
		return nil, 0, err
	}
	defer f.Close()

	var ring []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	total := 0
	for scanner.Scan() {
		total++
		line := scanner.Text()
		if len(ring) < maxLines {
			ring = append(ring, line)
		} else {
			copy(ring, ring[1:])
			ring[len(ring)-1] = line
		}
	}
	if err := scanner.Err(); err != nil {
		return ring, total, err
	}
	return ring, total, nil
}
