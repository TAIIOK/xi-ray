package server

import (
	"encoding/json"
	"net/http"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

func (s *Server) handleRouting(w http.ResponseWriter, r *http.Request) {
	http.ServeFileFS(w, r, staticFS, "routing.html")
}

func (s *Server) handleAPIRouting(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		data, err := s.panel.GetRouting()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, data)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleAPIUpdateRouting(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Routing config.Routing `json:"routing"`
		Apply   bool           `json:"apply"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	result, err := s.panel.UpdateRoutingWithApply(r.Context(), req.Routing, req.Apply)
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
