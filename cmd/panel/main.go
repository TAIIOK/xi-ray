package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/taiiok/xiaomi-vless/internal/config"
	"github.com/taiiok/xiaomi-vless/internal/server"
	"github.com/taiiok/xiaomi-vless/internal/service"
	"github.com/taiiok/xiaomi-vless/internal/setup"
	"github.com/taiiok/xiaomi-vless/internal/update"
	"github.com/taiiok/xiaomi-vless/internal/version"
)

func main() {
	configPath := flag.String("config", envOr("PANEL_CONFIG", defaultPanelConfigPath()), "path to panel.json")
	listenOverride := flag.String("listen", envOr("PANEL_LISTEN", ""), "override listen_addr (e.g. 0.0.0.0:7777 or 0.0.0.0 for local dev)")
	resetMode := flag.String("reset", "", "reset panel.json: onboarding (auth+setup+nodes) or full (factory, keeps paths)")
	showVersion := flag.Bool("version", false, "print version and exit")
	updateHome := flag.String("update-home", "", "panel home for updater CLI")
	updateSetPhase := flag.String("update-set-phase", "", "updater: set phase in state.json")
	updateGetPhase := flag.Bool("update-get-phase", false, "updater: print phase")
	updateApply := flag.Bool("update-apply", false, "updater: apply staged bundle")
	updateRollback := flag.Bool("update-rollback", false, "updater: rollback to panel.previous")
	postUpdate := flag.Bool("post-update", false, "run post-update migration and apply VPN stack")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.String())
		return
	}

	if *updateHome != "" {
		home := strings.TrimSpace(*updateHome)
		switch {
		case *updateSetPhase != "":
			if err := update.CLISetPhase(home, *updateSetPhase); err != nil {
				log.Fatalf("update set-phase: %v", err)
			}
			return
		case *updateGetPhase:
			if err := update.CLIGetPhase(home); err != nil {
				log.Fatalf("update get-phase: %v", err)
			}
			return
		case *updateApply:
			if err := update.CLIUpdaterApply(home); err != nil {
				log.Fatalf("update apply: %v", err)
			}
			return
		case *updateRollback:
			if err := update.CLIRollback(home); err != nil {
				log.Fatalf("update rollback: %v", err)
			}
			return
		}
	}

	store, err := config.NewStore(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if *postUpdate {
		panel := service.NewPanelService(store)
		setup.BootstrapOnStart(store, *configPath, false)
		if err := panel.RunPostUpdate(context.Background()); err != nil {
			log.Fatalf("post-update: %v", err)
		}
		log.Printf("post-update complete")
		return
	}

	if *resetMode != "" {
		mode := strings.TrimSpace(*resetMode)
		if err := service.ResetPanelConfig(store, mode); err != nil {
			log.Fatalf("reset: %v", err)
		}
		log.Printf("reset %q OK — login admin/admin, then open /onboarding", mode)
		return
	}

	panel := service.NewPanelService(store)

	localDev := *listenOverride != "" || os.Getenv("PANEL_DEV") == "1"
	if !localDev && !setup.PathWritable("/data") && !setup.PathWritable(store.Get().Paths.PanelDataDir) {
		localDev = true
	}
	setup.BootstrapOnStart(store, *configPath, localDev)

	cfg := store.Get()
	if err := config.EnsureDir(cfg.Paths.PanelDataDir, config.PanelDirPerm); err != nil {
		log.Fatalf("mkdir data dir: %v", err)
	}

	setupPanelLogging(cfg)

	upd := update.NewService(cfg.Paths.PanelDataDir, *configPath, store.Get)
	upd.PostUpdateHook = panel.RunPostUpdate
	if err := upd.ResumeOrVerify(context.Background()); err != nil {
		log.Printf("update resume: %v", err)
	}
	upd.StartHealthCheckIfNeeded(context.Background())

	wd := service.NewWatchdog(panel)
	wd.Start()

	subSched := service.NewSubscriptionScheduler(panel)
	subSched.Start()

	srv := server.New(panel, upd)
	addr := resolveListenAddr(*listenOverride, cfg.Network.ListenAddr)
	openURL := displayListenURL(addr)
	log.Printf("Xiaomi VLESS Panel listening on http://%s", addr)
	if panel.NeedsOnboarding() {
		log.Printf("Onboarding required — open http://%s/onboarding after login", openURL)
	}
	if *listenOverride != "" {
		log.Printf("Listen override active (-listen / PANEL_LISTEN); panel.json unchanged")
	}
	log.Printf("Default login: %s / admin (change password in onboarding or settings)", cfg.Auth.Username)
	if err := http.ListenAndServe(addr, srv.Router()); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func setupPanelLogging(cfg config.PanelConfig) {
	logPath := cfg.Logs.Panel
	if logPath == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(logPath), config.PanelDirPerm); err != nil {
		log.Printf("panel log dir: %v", err)
		return
	}
	_ = os.Chmod(filepath.Dir(logPath), config.PanelDirPerm)
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("panel log file: %v", err)
		return
	}
	log.SetOutput(io.MultiWriter(os.Stderr, f))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// resolveListenAddr applies -listen / PANEL_LISTEN without modifying panel.json.
// Accepts "host:port" or host only (port taken from config, default 7777).
func resolveListenAddr(override, fromConfig string) string {
	override = strings.TrimSpace(override)
	if override == "" {
		return fromConfig
	}
	if strings.Contains(override, ":") {
		return override
	}
	_, port, err := net.SplitHostPort(fromConfig)
	if err != nil || port == "" {
		port = "7777"
	}
	return net.JoinHostPort(override, port)
}

func displayListenURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		return net.JoinHostPort("127.0.0.1", port)
	}
	return addr
}

func defaultPanelConfigPath() string {
	for _, m := range setup.DiscoverUSBMounts() {
		return config.PanelConfigPathOnUSB(m.Path)
	}
	return config.PanelConfigPathOnUSB(config.DefaultUSBMount)
}
