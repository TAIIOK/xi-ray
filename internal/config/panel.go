package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const CurrentVersion = 2

type PanelConfig struct {
	Version             int                 `json:"version"`
	Paths               Paths               `json:"paths"`
	Network             Network             `json:"network"`
	Iptables            Iptables            `json:"iptables"`
	Auth                Auth                `json:"auth"`
	Setup               Setup               `json:"setup"`
	Subscriptions       []Subscription      `json:"subscriptions"`
	SubscriptionsPolicy SubscriptionsPolicy `json:"subscriptions_policy"`
	Nodes               []Node              `json:"nodes"`
	Selection           Selection           `json:"selection"`
	Observatory         Observatory         `json:"observatory"`
	Watchdog            Watchdog            `json:"watchdog"`
	Logs                Logs                `json:"logs"`
	Routing             Routing             `json:"routing"`
}

type Paths struct {
	XrayBin        string `json:"xray_bin"`
	XrayConfig     string `json:"xray_config"`
	StartupScript  string `json:"startup_script"`
	IptablesScript string `json:"iptables_script"`
	PanelDataDir   string `json:"panel_data_dir"`
}

type Network struct {
	GuestSubnet string `json:"guest_subnet"`
	ListenAddr  string `json:"listen_addr"`
	XrayAPIAddr string `json:"xray_api_addr"`
}

type Iptables struct {
	GuestSubnet string `json:"guest_subnet"`
	TCPPort     int    `json:"tcp_port"`
	UDPPort     int    `json:"udp_port"`
	SOCKSPort   int    `json:"socks_port"`
	APIPort     int    `json:"api_port"`
}

type Setup struct {
	OnboardingCompleted bool `json:"onboarding_completed"`
}

type Watchdog struct {
	Enabled                      bool   `json:"enabled"`
	IntervalSec                  int    `json:"interval_sec"`
	RefreshSubscriptionsOnOutage bool   `json:"refresh_subscriptions_on_outage"`
	TelegramBotToken             string `json:"telegram_bot_token,omitempty"`
	TelegramChatID               string `json:"telegram_chat_id,omitempty"`
	LastAlert                    string `json:"last_alert,omitempty"`
}

type Logs struct {
	XrayAccess string `json:"xray_access_log"`
	XrayError  string `json:"xray_error_log"`
	Startup    string `json:"startup_log"`
	Panel      string `json:"panel_log"`
}

type Auth struct {
	Username     string `json:"username"`
	PasswordHash string `json:"password_hash"`
}

type Subscription struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SubscriptionsPolicy controls periodic refresh and auto-selection after updates.
type SubscriptionsPolicy struct {
	AutoRefreshEnabled     bool      `json:"auto_refresh_enabled"`
	AutoRefreshIntervalMin int       `json:"auto_refresh_interval_min"`
	ReselectStrategy       string    `json:"reselect_strategy"` // keep | first | best_ping
	AutoApplyOnChange      bool      `json:"auto_apply_on_change"`
	LastAutoRefreshAt      time.Time `json:"last_auto_refresh_at,omitempty"`
}

type Node struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Protocol       string         `json:"protocol"`
	Address        string         `json:"address"`
	Port           int            `json:"port"`
	UUID           string         `json:"uuid"`
	Security       string         `json:"security"`
	Network        string         `json:"network"`
	Flow           string         `json:"flow,omitempty"`
	SNI            string         `json:"sni,omitempty"`
	Fingerprint    string         `json:"fingerprint,omitempty"`
	PublicKey      string         `json:"public_key,omitempty"`
	ShortID        string         `json:"short_id,omitempty"`
	SpiderX        string         `json:"spider_x,omitempty"`
	Path           string         `json:"path,omitempty"`
	Host           string         `json:"host,omitempty"`
	XHTTPMode      string         `json:"xhttp_mode,omitempty"`
	XHTTPExtra     map[string]any `json:"xhttp_extra,omitempty"`
	Password       string         `json:"password,omitempty"`
	AlterID        int            `json:"alter_id,omitempty"`
	Method         string         `json:"method,omitempty"`
	SubscriptionID string         `json:"subscription_id,omitempty"`
	Manual         bool           `json:"manual"`
	RawLink        string         `json:"raw_link,omitempty"`
	Hash           string         `json:"hash"`
	LastLatencyMs  int            `json:"last_latency_ms,omitempty"`
	LastHealth     string         `json:"last_health,omitempty"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type Selection struct {
	Mode          string   `json:"mode"`
	ActiveNodeIDs []string `json:"active_node_ids"`
	FallbackOrder []string `json:"fallback_order"`
}

type Observatory struct {
	Enabled       bool   `json:"enabled"`
	ProbeURL      string `json:"probe_url"`
	ProbeInterval string `json:"probe_interval"`
}

type Store struct {
	path string
	mu   sync.RWMutex
	cfg  PanelConfig
}

func DefaultPanelConfig() PanelConfig {
	hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	return PanelConfig{
		Version: CurrentVersion,
		Paths:   PathsForUSB(DefaultUSBMount),
		Network: Network{
			GuestSubnet: "192.168.33.0/24",
			ListenAddr:  "192.168.31.1:7777",
			XrayAPIAddr: "127.0.0.1:10085",
		},
		Iptables: Iptables{
			GuestSubnet: "192.168.33.0/24",
			TCPPort:     12346,
			UDPPort:     12345,
			SOCKSPort:   10808,
			APIPort:     10085,
		},
		Auth: Auth{
			Username:     "admin",
			PasswordHash: string(hash),
		},
		Setup:         Setup{OnboardingCompleted: false},
		Subscriptions: []Subscription{},
		SubscriptionsPolicy: SubscriptionsPolicy{
			AutoRefreshEnabled:     true,
			AutoRefreshIntervalMin: 60,
			ReselectStrategy:       "best_ping",
			AutoApplyOnChange:      true,
		},
		Nodes: []Node{},
		Selection: Selection{
			Mode:          "single",
			ActiveNodeIDs: []string{},
			FallbackOrder: []string{},
		},
		Observatory: Observatory{
			Enabled:       true,
			ProbeURL:      "https://www.google.com/generate_204",
			ProbeInterval: "30s",
		},
		Watchdog: Watchdog{
			Enabled:                      true,
			IntervalSec:                  60,
			RefreshSubscriptionsOnOutage: true,
		},
		Logs:    LogsForPanelHome(PanelHomeOnUSB(DefaultUSBMount)),
		Routing: DefaultRouting(),
	}
}

func (c *PanelConfig) Normalize() {
	if c.Iptables.GuestSubnet == "" {
		c.Iptables.GuestSubnet = c.Network.GuestSubnet
	}
	if c.Network.GuestSubnet == "" {
		c.Network.GuestSubnet = c.Iptables.GuestSubnet
	}
	if c.Iptables.TCPPort == 0 {
		c.Iptables.TCPPort = 12346
	}
	if c.Iptables.UDPPort == 0 {
		c.Iptables.UDPPort = 12345
	}
	if c.Iptables.SOCKSPort == 0 {
		c.Iptables.SOCKSPort = 10808
	}
	if c.Iptables.APIPort == 0 {
		c.Iptables.APIPort = 10085
	}
	if c.Network.XrayAPIAddr == "" {
		c.Network.XrayAPIAddr = fmt.Sprintf("127.0.0.1:%d", c.Iptables.APIPort)
	}
	if c.Watchdog.IntervalSec == 0 {
		c.Watchdog.IntervalSec = 60
	}
	if c.SubscriptionsPolicy.AutoRefreshIntervalMin == 0 {
		c.SubscriptionsPolicy.AutoRefreshIntervalMin = 60
	}
	switch c.SubscriptionsPolicy.ReselectStrategy {
	case "", "keep", "first", "best_ping":
		if c.SubscriptionsPolicy.ReselectStrategy == "" {
			c.SubscriptionsPolicy.ReselectStrategy = "best_ping"
		}
	default:
		c.SubscriptionsPolicy.ReselectStrategy = "best_ping"
	}
	if c.Logs.Startup == "" {
		c.Logs.Startup = filepath.Join(c.Paths.PanelDataDir, "xray-startup.log")
	}
	if c.Logs.Panel == "" {
		c.Logs.Panel = filepath.Join(c.Paths.PanelDataDir, "panel.log")
	}
	if c.Logs.XrayAccess == "" {
		c.Logs.XrayAccess = filepath.Join(c.Paths.PanelDataDir, "xray-access.log")
	}
	if c.Logs.XrayError == "" {
		c.Logs.XrayError = filepath.Join(c.Paths.PanelDataDir, "xray-error.log")
	}
	if c.Paths.PanelDataDir == "" {
		if usb := USBMountFromXrayBin(c.Paths.XrayBin); usb != "" {
			c.Paths.PanelDataDir = PanelHomeOnUSB(usb)
		}
	}
	c.Routing.Normalize()
	c.Version = CurrentVersion
	ensureAuthDefaults(&c.Auth)
}

// ensureAuthDefaults fills missing auth fields (fresh install from panel.json.example).
func ensureAuthDefaults(auth *Auth) {
	if auth.Username == "" {
		auth.Username = "admin"
	}
	if strings.TrimSpace(auth.PasswordHash) == "" {
		hash, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		if err == nil {
			auth.PasswordHash = string(hash)
		}
	}
}

func (s *Store) IsDefaultPassword() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return bcrypt.CompareHashAndPassword([]byte(s.cfg.Auth.PasswordHash), []byte("admin")) == nil
}

func NewStore(path string) (*Store, error) {
	s := &Store{path: path}
	if err := s.Load(); err != nil {
		if os.IsNotExist(err) {
			s.cfg = DefaultPanelConfig()
			if err := s.Save(); err != nil {
				return nil, err
			}
			return s, nil
		}
		return nil, err
	}
	return s, nil
}

func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	var cfg PanelConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	emptyPassword := strings.TrimSpace(cfg.Auth.PasswordHash) == ""
	cfg.Normalize()
	migrated := false
	if emptyPassword && cfg.Auth.PasswordHash != "" {
		migrated = true
	}
	if MigrateLegacyDataPaths(&cfg) {
		migrated = true
	}
	if cfg.Network.ListenAddr == "192.168.31.1:8080" {
		cfg.Network.ListenAddr = "192.168.31.1:7777"
		migrated = true
	}
	if !cfg.Setup.OnboardingCompleted && strings.TrimSpace(cfg.Auth.PasswordHash) != "" {
		if bcrypt.CompareHashAndPassword([]byte(cfg.Auth.PasswordHash), []byte("admin")) != nil {
			cfg.Setup.OnboardingCompleted = true
		}
	}
	s.cfg = cfg
	if !migrated {
		return nil
	}
	data, err = json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) Replace(cfg PanelConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg.Normalize()
	s.cfg = cfg
}

func (s *Store) Save() error {
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) Get() PanelConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg := s.cfg
	cfg.Normalize()
	return cfg
}

func (s *Store) Update(fn func(*PanelConfig) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := fn(&s.cfg); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) NodeByID(id string) (Node, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.NodeByID(id)
}

func (c *PanelConfig) NodeByID(id string) (Node, bool) {
	for _, n := range c.Nodes {
		if n.ID == id {
			return n, true
		}
	}
	return Node{}, false
}

func (s *Store) CheckPassword(username, password string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cfg.Auth.Username != username {
		return false
	}
	if strings.TrimSpace(s.cfg.Auth.PasswordHash) == "" {
		return password == "admin"
	}
	return bcrypt.CompareHashAndPassword([]byte(s.cfg.Auth.PasswordHash), []byte(password)) == nil
}

func (s *Store) SetPassword(username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.Update(func(cfg *PanelConfig) error {
		if username != "" {
			cfg.Auth.Username = username
		}
		cfg.Auth.PasswordHash = string(hash)
		return nil
	})
}

func ValidatePaths(p Paths) error {
	for _, path := range []string{p.XrayBin, p.XrayConfig, p.StartupScript, p.IptablesScript} {
		if path == "" {
			return fmt.Errorf("path must not be empty")
		}
		if filepath.Clean(path) != path || containsTraversal(path) {
			return fmt.Errorf("invalid path: %s", path)
		}
	}
	return nil
}

func containsTraversal(path string) bool {
	return len(path) >= 2 && (path[:2] == ".." || path[len(path)-2:] == "..")
}
