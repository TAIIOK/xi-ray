package server

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/taiiok/xiaomi-vless/internal/config"
	"github.com/taiiok/xiaomi-vless/internal/i18n"
	"github.com/taiiok/xiaomi-vless/internal/service"
	"github.com/taiiok/xiaomi-vless/internal/update"
)

//go:embed static/*
var staticFiles embed.FS

var staticFS fs.FS

func init() {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	staticFS = sub
}

type Server struct {
	panel  *service.PanelService
	auth   *Auth
	update *update.Service
}

func New(panel *service.PanelService, upd *update.Service) *Server {
	return &Server{
		panel:  panel,
		auth:   NewAuth(panel.Store()),
		update: upd,
	}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	r.Get("/login", s.auth.LoginHandler)
	r.Post("/login", s.auth.LoginHandler)
	r.Get("/logout", s.auth.LogoutHandler)
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	r.Group(func(r chi.Router) {
		r.Use(s.auth.Middleware)
		r.Use(s.onboardingMiddleware)
		r.Get("/", s.handleIndex)
		r.Get("/servers", s.handleServers)
		r.Get("/subscriptions", s.handleSubscriptions)
		r.Get("/settings", s.handleSettings)
		r.Get("/routing", s.handleRouting)
		r.Get("/logs", s.handleLogs)
		r.Get("/onboarding", s.handleOnboarding)

		r.Get("/api/status", s.handleAPIStatus)
		r.Get("/api/nodes", s.handleAPINodes)
		r.Get("/api/settings", s.handleAPISettings)
		r.Get("/api/subscriptions-list", s.handleAPISubscriptionsList)
		r.Get("/api/observatory", s.handleAPIObservatory)
		r.Get("/api/onboarding/status", s.handleAPIOnboardingStatus)
		r.Post("/api/onboarding/complete", s.handleAPIOnboardingComplete)
		r.Post("/api/onboarding/skip", s.handleAPIOnboardingSkip)
		r.Put("/api/onboarding/paths", s.handleAPIOnboardingPaths)
		r.Post("/api/onboarding/verify", s.handleAPIOnboardingVerify)
		r.Post("/api/onboarding/setup", s.handleAPIOnboardingSetup)
		r.Post("/api/onboarding/download-xray", s.handleAPIOnboardingDownloadXray)
		r.Post("/api/onboarding/import", s.handleAPIOnboardingImport)
		r.Put("/api/onboarding/selection", s.handleAPIOnboardingSelection)
		r.Put("/api/onboarding/network", s.handleAPIOnboardingNetwork)
		r.Get("/api/network/guest-check", s.handleAPIGuestNetworkCheck)
		r.Post("/api/network/guest-check", s.handleAPIGuestNetworkCheck)
		r.Get("/api/logs", s.handleAPILogs)
		r.Get("/api/backup", s.handleAPIBackup)
		r.Post("/api/restore", s.handleAPIRestore)
		r.Post("/api/subscriptions", s.handleAPIAddSubscription)
		r.Delete("/api/subscriptions/{id}", s.handleAPIDeleteSubscription)
		r.Post("/api/subscriptions/{id}/refresh", s.handleAPIRefreshSubscription)
		r.Post("/api/nodes/manual", s.handleAPIAddManualNode)
		r.Delete("/api/nodes/{id}", s.handleAPIDeleteNode)
		r.Put("/api/selection", s.handleAPIUpdateSelection)
		r.Post("/api/apply", s.handleAPIApply)
		r.Post("/api/restart", s.handleAPIRestart)
		r.Post("/api/nodes/probe", s.handleAPIProbeNodes)
		r.Post("/api/nodes/{id}/ping", s.handleAPIPingNode)
		r.Post("/api/watchdog/dismiss", s.handleAPIWatchdogDismiss)
		r.Put("/api/settings", s.handleAPIUpdateSettings)
		r.Get("/api/routing", s.handleAPIRouting)
		r.Put("/api/routing", s.handleAPIUpdateRouting)
		r.Put("/api/auth/password", s.handleAPIUpdatePassword)

		r.Get("/api/xray/status", s.handleAPIXrayStatus)
		r.Post("/api/xray/update", s.handleAPIXrayUpdate)

		r.Get("/api/update/status", s.handleAPIUpdateStatus)
		r.Get("/api/update/check", s.handleAPIUpdateCheck)
		r.Post("/api/update/download", s.handleAPIUpdateDownload)
		r.Post("/api/update/cancel", s.handleAPIUpdateCancel)
		r.Post("/api/update/apply", s.handleAPIUpdateApply)
		r.Post("/api/update/rollback", s.handleAPIUpdateRollback)
		r.Post("/api/update/confirm", s.handleAPIUpdateConfirm)
	})
	return r
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFileFS(w, r, staticFS, "index.html")
}

func (s *Server) handleServers(w http.ResponseWriter, r *http.Request) {
	http.ServeFileFS(w, r, staticFS, "servers.html")
}

func (s *Server) handleSubscriptions(w http.ResponseWriter, r *http.Request) {
	http.ServeFileFS(w, r, staticFS, "subscriptions.html")
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	http.ServeFileFS(w, r, staticFS, "settings.html")
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	http.ServeFileFS(w, r, staticFS, "logs.html")
}

func (s *Server) handleOnboarding(w http.ResponseWriter, r *http.Request) {
	http.ServeFileFS(w, r, staticFS, "onboarding.html")
}

func (s *Server) onboardingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.panel.NeedsOnboarding() {
			if r.URL.Path == "/onboarding" {
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		if isOnboardingAllowed(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if isAPI(r) {
			locale := i18n.LocaleFromRequest(r)
			writeJSON(w, http.StatusForbidden, map[string]string{"error": i18n.T(locale, "onboarding required")})
			return
		}
		http.Redirect(w, r, "/onboarding", http.StatusSeeOther)
	})
}

func isOnboardingAllowed(path string) bool {
	switch path {
	case "/onboarding", "/logout", "/routing", "/api/routing", "/api/onboarding/status", "/api/onboarding/complete",
		"/api/onboarding/skip", "/api/onboarding/paths", "/api/onboarding/verify", "/api/onboarding/setup",
		"/api/onboarding/download-xray", "/api/onboarding/import", "/api/onboarding/selection",
		"/api/onboarding/network", "/api/network/guest-check",
		"/api/settings", "/api/nodes", "/api/apply",
		"/api/selection", "/api/nodes/manual", "/api/subscriptions", "/api/subscriptions-list":
		return true
	}
	if path == "/api/settings" || strings.HasPrefix(path, "/api/subscriptions/") {
		return true
	}
	if strings.HasPrefix(path, "/static/") {
		return true
	}
	return false
}

func (s *Server) handleAPIStatus(w http.ResponseWriter, r *http.Request) {
	resp := s.panel.Status().GetStatus(r.Context())
	locale := i18n.LocaleFromRequest(r)
	if resp.Message != "" {
		resp.Message = i18n.T(locale, resp.Message)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAPINodes(w http.ResponseWriter, r *http.Request) {
	cfg := s.panel.Store().Get()
	writeJSON(w, http.StatusOK, map[string]any{
		"nodes":     cfg.Nodes,
		"selection": cfg.Selection,
	})
}

func (s *Server) handleAPISettings(w http.ResponseWriter, r *http.Request) {
	cfg := s.panel.Store().Get()
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) handleAPISubscriptionsList(w http.ResponseWriter, r *http.Request) {
	cfg := s.panel.Store().Get()
	items := make([]map[string]any, 0, len(cfg.Subscriptions))
	for _, sub := range cfg.Subscriptions {
		nodeCount := 0
		for _, n := range cfg.Nodes {
			if n.SubscriptionID == sub.ID {
				nodeCount++
			}
		}
		items = append(items, map[string]any{
			"id":         sub.ID,
			"name":       sub.Name,
			"url":        sub.URL,
			"updated_at": sub.UpdatedAt,
			"node_count": nodeCount,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"subscriptions": items,
		"total_nodes":   len(cfg.Nodes),
	})
}

func (s *Server) handleAPIObservatory(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.panel.Status().GetObservatory(r.Context()))
}

func (s *Server) handleAPIAddSubscription(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.URL == "" {
		locale := i18n.LocaleFromRequest(r)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, "url required")})
		return
	}
	if req.Name == "" {
		req.Name = "Subscription"
	}
	sub, nodes, err := s.panel.AddSubscription(r.Context(), req.Name, req.URL)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"subscription": sub, "nodes": nodes})
}

func (s *Server) handleAPIDeleteSubscription(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.panel.DeleteSubscription(id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (s *Server) handleAPIRefreshSubscription(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	result, err := s.panel.RefreshSubscriptionManaged(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAPIAddManualNode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Link string `json:"link"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	node, err := s.panel.AddManualNode(strings.TrimSpace(req.Link))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) handleAPIDeleteNode(w http.ResponseWriter, r *http.Request) {
	if err := s.panel.DeleteNode(chi.URLParam(r, "id")); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (s *Server) handleAPIUpdateSelection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Mode          string   `json:"mode"`
		ActiveNodeIDs []string `json:"active_node_ids"`
		FallbackOrder []string `json:"fallback_order"`
		Apply         bool     `json:"apply"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	result, err := s.panel.UpdateSelectionWithApply(r.Context(), req.Mode, req.ActiveNodeIDs, req.FallbackOrder, req.Apply)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	locale := i18n.LocaleFromRequest(r)
	if result.Message != "" {
		result.Message = i18n.T(locale, result.Message)
	}
	status := http.StatusOK
	if !result.OK {
		status = http.StatusBadRequest
	}
	writeJSON(w, status, result)
}

func (s *Server) handleAPIApply(w http.ResponseWriter, r *http.Request) {
	res, err := s.panel.Apply().Apply(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	locale := i18n.LocaleFromRequest(r)
	if res.Message != "" {
		res.Message = i18n.T(locale, res.Message)
	}
	status := http.StatusOK
	if !res.OK {
		status = http.StatusBadRequest
	}
	writeJSON(w, status, res)
}

func (s *Server) handleAPIRestart(w http.ResponseWriter, r *http.Request) {
	if err := s.panel.Apply().Restart(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (s *Server) handleAPIProbeNodes(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NodeIDs []string `json:"node_ids"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := s.panel.Status().ProbeNodes(r.Context(), req.NodeIDs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, s.panel.Status().GetObservatory(r.Context()))
}

func (s *Server) handleAPIPingNode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	result, err := s.panel.Status().PingNode(r.Context(), id)
	if err != nil {
		locale := i18n.LocaleFromRequest(r)
		msg := err.Error()
		if msg == "node not found" {
			msg = i18n.T(locale, "node not found")
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAPIWatchdogDismiss(w http.ResponseWriter, r *http.Request) {
	if err := s.panel.DismissWatchdogAlert(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (s *Server) handleAPIUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Paths               config.Paths               `json:"paths"`
		Network             config.Network             `json:"network"`
		Iptables            config.Iptables            `json:"iptables"`
		Observatory         config.Observatory         `json:"observatory"`
		Watchdog            config.Watchdog            `json:"watchdog"`
		FailOpen            config.FailOpen            `json:"fail_open"`
		SubscriptionsPolicy config.SubscriptionsPolicy `json:"subscriptions_policy"`
		Logs                config.Logs                `json:"logs"`
		Routing             config.Routing             `json:"routing"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.panel.UpdateSettings(req.Paths, req.Network, req.Iptables, req.Observatory, req.Watchdog, req.FailOpen, req.SubscriptionsPolicy, req.Logs, req.Routing); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (s *Server) handleAPIUpdatePassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username        string `json:"username"`
		Password        string `json:"password"`
		CurrentPassword string `json:"current_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	store := s.panel.Store()
	cfg := store.Get()
	if !store.CheckPassword(cfg.Auth.Username, req.CurrentPassword) {
		locale := i18n.LocaleFromRequest(r)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": i18n.T(locale, "invalid current password")})
		return
	}
	if req.Password == "" {
		locale := i18n.LocaleFromRequest(r)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, "password required")})
		return
	}
	if err := store.SetPassword(req.Username, req.Password); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}
