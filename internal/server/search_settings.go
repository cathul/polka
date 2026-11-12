package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/vestigiumincaligne/polka/internal/store"
)

func webSearchLanguage(r *http.Request) string {
	return store.SearchLanguage(r.URL.Query().Get("searchLanguage"))
}

func (s *Server) opdsSearchLanguage(r *http.Request) string {
	if user := s.currentUser(r); user != nil {
		return store.SearchLanguage(s.users.GetSetting(r.Context(), fmt.Sprintf("opds.search_language:%d", user.ID), ""))
	}
	return ""
}

// A personal preference consumed exclusively by OPDS search.
func (s *Server) handleSearchSettings(w http.ResponseWriter, r *http.Request) {
	user := s.currentUser(r)
	if user == nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	if r.Method == http.MethodPost {
		var input struct {
			Language string `json:"opdsSearchLanguage"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		value := strings.ToLower(strings.TrimSpace(input.Language))
		if value != "" && store.SearchLanguage(value) == "" {
			http.Error(w, "invalid language", http.StatusBadRequest)
			return
		}
		if err := s.users.SetSetting(r.Context(), fmt.Sprintf("opds.search_language:%d", user.ID), value); err != nil {
			s.apiError(w, err)
			return
		}
	}
	writeJSON(w, map[string]string{"opdsSearchLanguage": s.opdsSearchLanguage(r)})
}
