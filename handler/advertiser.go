package handler

import (
	"vectorspace/platform"
	"encoding/json"
	"net/http"
	"strings"
)

type registerRequest struct {
	Name     string  `json:"name"`
	Intent   string  `json:"intent"`
	Sigma    float64 `json:"sigma"`
	BidPrice float64 `json:"bid_price"`
	Budget   float64 `json:"budget"`
	Currency string  `json:"currency"`
	URL      string  `json:"url"`
}

type updateRequest struct {
	Name     string  `json:"name"`
	Intent   string  `json:"intent"`
	Sigma    float64 `json:"sigma"`
	BidPrice float64 `json:"bid_price"`
	Budget   float64 `json:"budget"`
	URL      string  `json:"url"`
}

type AdvertiserHandler struct {
	Registry *platform.PositionRegistry
	Budgets  *platform.BudgetTracker
	DB       *platform.DB
}

// HandleRegister handles POST /advertiser/register
func (h *AdvertiserHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if req.Intent == "" {
		http.Error(w, "intent is required", http.StatusBadRequest)
		return
	}
	if req.BidPrice <= 0 {
		http.Error(w, "bid_price must be positive", http.StatusBadRequest)
		return
	}
	if req.Budget <= 0 {
		http.Error(w, "budget must be positive", http.StatusBadRequest)
		return
	}
	if req.Sigma <= 0 {
		http.Error(w, "sigma must be positive", http.StatusBadRequest)
		return
	}
	if req.Currency == "" {
		req.Currency = "USD"
	}

	pos, err := h.Registry.RegisterWithBudget(req.Name, req.Intent, req.Sigma, req.BidPrice, req.Budget, req.Currency, req.URL)
	if err != nil {
		http.Error(w, "registration failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	h.Budgets.Set(pos.ID, req.Budget, pos.Currency)

	// Generate access token if DB is available
	resp := map[string]interface{}{
		"id":        pos.ID,
		"name":      pos.Name,
		"intent":    pos.Intent,
		"sigma":     pos.Sigma,
		"bid_price": pos.BidPrice,
		"currency":  pos.Currency,
		"url":       pos.URL,
	}
	if h.DB != nil {
		if token, err := h.DB.GenerateToken(pos.ID); err == nil {
			resp["token"] = token
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// HandlePositions handles GET /positions
func (h *AdvertiserHandler) HandlePositions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	positions := h.Registry.GetAll()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(positions)
}

// HandleBudget handles GET /budget/{id}
func (h *AdvertiserHandler) HandleBudget(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/budget/")
	if id == "" {
		http.Error(w, "advertiser id is required", http.StatusBadRequest)
		return
	}

	info := h.Budgets.GetInfo(id)
	if info == nil {
		http.Error(w, "advertiser not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

// HandleAdvertiser handles PUT /advertiser/{id} and DELETE /advertiser/{id}
func (h *AdvertiserHandler) HandleAdvertiser(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/advertiser/")
	if id == "" || id == "register" {
		http.Error(w, "advertiser id is required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPut:
		h.handleUpdate(w, r, id)
	case http.MethodDelete:
		h.handleDelete(w, r, id)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *AdvertiserHandler) handleUpdate(w http.ResponseWriter, r *http.Request, id string) {
	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	pos, err := h.Registry.Update(id, req.Name, req.Intent, req.URL, req.Sigma, req.BidPrice)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if req.Budget > 0 && h.DB != nil {
		if err := h.DB.UpdateBudget(id, req.Budget); err != nil {
			http.Error(w, "failed to update budget: "+err.Error(), http.StatusInternalServerError)
			return
		}
		h.Budgets.Set(id, req.Budget, pos.Currency)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pos)
}

func (h *AdvertiserHandler) handleDelete(w http.ResponseWriter, _ *http.Request, id string) {
	if err := h.Registry.Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	h.Budgets.Delete(id)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "id": id})
}
