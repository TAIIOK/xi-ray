package server

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleAPIUpdateStatus(w http.ResponseWriter, r *http.Request) {
	st, err := s.update.Status(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleAPIUpdateCheck(w http.ResponseWriter, r *http.Request) {
	st, err := s.update.Check(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleAPIUpdateDownload(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Version string `json:"version"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	st, err := s.update.Download(r.Context(), req.Version)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	status := http.StatusOK
	if st.Error != "" {
		status = http.StatusBadRequest
	}
	writeJSON(w, status, st)
}

func (s *Server) handleAPIUpdateCancel(w http.ResponseWriter, r *http.Request) {
	st, err := s.update.Cancel(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleAPIUpdateApply(w http.ResponseWriter, r *http.Request) {
	st, err := s.update.Apply(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleAPIUpdateRollback(w http.ResponseWriter, r *http.Request) {
	st, err := s.update.Rollback(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleAPIUpdateConfirm(w http.ResponseWriter, r *http.Request) {
	st, err := s.update.Confirm(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, st)
}
