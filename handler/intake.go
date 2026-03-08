package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"vectorspace/platform"
)

// IntakeHandler handles intake form submissions from the landing page.
type IntakeHandler struct {
	DB *platform.DB
}

type intakeRequest struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	Company     string `json:"company"`
	Detail      string `json:"detail"`
	Description string `json:"description"`
}

func (h *IntakeHandler) HandleSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		subs, err := h.DB.GetIntakeSubmissions()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(subs)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req intakeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	req.Type = strings.TrimSpace(req.Type)
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(req.Email)

	if req.Type == "" || req.Name == "" || req.Email == "" {
		http.Error(w, "type, name, and email are required", http.StatusBadRequest)
		return
	}
	if req.Type != "publisher" && req.Type != "advertiser" {
		http.Error(w, "type must be 'publisher' or 'advertiser'", http.StatusBadRequest)
		return
	}

	id, err := h.DB.InsertIntakeSubmission(req.Type, req.Name, req.Email, req.Company, req.Detail, req.Description)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"id": id})
}
