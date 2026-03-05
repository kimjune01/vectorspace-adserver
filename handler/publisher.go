package handler

import (
	"cloudx-adserver/platform"
	"encoding/json"
	"net/http"
)

type adRequestBody struct {
	Intent string  `json:"intent"`
	Tau    float64 `json:"tau,omitempty"`
}

type PublisherHandler struct {
	Engine *platform.AuctionEngine
}

// HandleAdRequest handles POST /ad-request
func (h *PublisherHandler) HandleAdRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req adRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Intent == "" {
		http.Error(w, "intent is required", http.StatusBadRequest)
		return
	}

	resp, err := h.Engine.RunAdRequestWithTau(req.Intent, req.Tau)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
