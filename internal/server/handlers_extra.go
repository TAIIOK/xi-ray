package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/taiiok/xiaomi-vless/internal/config"
	"github.com/taiiok/xiaomi-vless/internal/service"
	"github.com/taiiok/xiaomi-vless/internal/setup"
)

func (s *Server) handleAPIOnboardingStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.panel.GetOnboardingStatus())
}

func (s *Server) handleAPIOnboardingComplete(w http.ResponseWriter, r *http.Request) {
	var req service.CompleteOnboardingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	result, err := s.panel.CompleteOnboarding(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	status := http.StatusOK
	if !result.OK {
		status = http.StatusBadRequest
	}
	writeJSON(w, status, result)
}

func (s *Server) handleAPIOnboardingSkip(w http.ResponseWriter, r *http.Request) {
	if err := s.panel.SkipOnboarding(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (s *Server) handleAPIOnboardingPaths(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Paths config.Paths `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	checks, err := s.panel.UpdateOnboardingPaths(req.Paths)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"path_checks": checks,
		"ready":       setup.AllChecksOK(checks),
		"paths":       req.Paths,
	})
}

func (s *Server) handleAPIOnboardingVerify(w http.ResponseWriter, r *http.Request) {
	st := s.panel.VerifyOnboardingPaths()
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleAPIOnboardingSetup(w http.ResponseWriter, r *http.Request) {
	result := s.panel.SetupOnboardingEnvironment()
	status := http.StatusOK
	if !result.OK {
		status = http.StatusBadRequest
	}
	writeJSON(w, status, result)
}

func (s *Server) handleAPIOnboardingDownloadXray(w http.ResponseWriter, r *http.Request) {
	var req service.DownloadXrayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	result := s.panel.DownloadXrayOnboarding(r.Context(), req)
	status := http.StatusOK
	if !result.OK {
		status = http.StatusBadRequest
	}
	writeJSON(w, status, result)
}

func (s *Server) handleAPIOnboardingImport(w http.ResponseWriter, r *http.Request) {
	var req service.ImportOnboardingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	result, err := s.panel.ImportOnboardingInput(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAPIOnboardingSelection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Mode          string   `json:"mode"`
		ActiveNodeIDs []string `json:"active_node_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.panel.SaveOnboardingSelection(req.Mode, req.ActiveNodeIDs); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	cfg := s.panel.Store().Get()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"selection": cfg.Selection,
	})
}

func (s *Server) handleAPILogs(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	lines := 200
	if v := r.URL.Query().Get("lines"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			lines = n
		}
	}
	resp, err := s.panel.GetLogs(source, lines)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAPIBackup(w http.ResponseWriter, r *http.Request) {
	data, err := s.panel.ExportBackup()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="panel-backup.json"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) handleAPIRestore(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(io.LimitReader(r.Body, 5<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if len(data) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "empty backup"})
		return
	}
	if err := s.panel.RestoreBackup(data); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (s *Server) handleAPIGuestNetworkCheck(w http.ResponseWriter, r *http.Request) {
	subnet := ""
	if r.Method == http.MethodPost {
		var req struct {
			GuestSubnet string `json:"guest_subnet"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		subnet = req.GuestSubnet
	}
	st := s.panel.CheckGuestNetwork(subnet)
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleAPIOnboardingNetwork(w http.ResponseWriter, r *http.Request) {
	var req struct {
		GuestSubnet string `json:"guest_subnet"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	st, err := s.panel.UpdateOnboardingNetwork(req.GuestSubnet)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":            true,
		"guest_subnet":  st.ConfigSubnet,
		"guest_network": st,
	})
}

func (s *Server) handleAPIXrayStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.panel.XrayStatus())
}

func (s *Server) handleAPIXrayUpdate(w http.ResponseWriter, r *http.Request) {
	var req service.UpdateXrayRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	result := s.panel.UpdateXray(r.Context(), req)
	status := http.StatusOK
	if !result.OK {
		status = http.StatusBadRequest
	}
	writeJSON(w, status, result)
}
