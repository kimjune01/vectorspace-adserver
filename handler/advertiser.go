package handler

import (
	"vectorspace/platform"
	"encoding/json"
	"net/http"
	"strings"
)

type registerRequest struct {
	Name string `json:"name"`
	// Intent is a natural-language positioning statement, embedded to a
	// single center. Mutually exclusive with Keywords.
	Intent string `json:"intent"`
	// Keywords is the import path for keyword campaigns: one position per
	// keyword, one shared budget. Sigma defaults to 0 (the exact-match
	// limit), so imported keywords behave as they did on the source platform.
	Keywords []string `json:"keywords"`
	Sigma    float64  `json:"sigma"`
	BidPrice float64  `json:"bid_price"`
	Budget   float64  `json:"budget"`
	Currency string   `json:"currency"`
	URL      string   `json:"url"`
}

type updateRequest struct {
	Name   string `json:"name"`
	Intent string `json:"intent"`
	// Sigma is a pointer so that an omitted field (keep the current value)
	// is distinguishable from an explicit 0 (the keyword limit).
	Sigma    *float64 `json:"sigma"`
	BidPrice float64  `json:"bid_price"`
	Budget   float64  `json:"budget"`
	URL      string   `json:"url"`
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
	if req.Intent == "" && len(req.Keywords) == 0 {
		http.Error(w, "intent or keywords is required", http.StatusBadRequest)
		return
	}
	if req.Intent != "" && len(req.Keywords) > 0 {
		http.Error(w, "intent and keywords are mutually exclusive", http.StatusBadRequest)
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
	if req.Sigma < 0 {
		http.Error(w, "sigma must not be negative", http.StatusBadRequest)
		return
	}
	// σ = 0 is the exact-match limit — a deliberate choice for keyword
	// imports, a silent footgun for intent positions (the circle would
	// never contain a live query). Require it explicitly positive there.
	if req.Intent != "" && req.Sigma == 0 {
		http.Error(w, "sigma must be positive for intent positions (keywords default to 0)", http.StatusBadRequest)
		return
	}
	if req.Currency == "" {
		req.Currency = "USD"
	}

	if len(req.Keywords) > 0 {
		h.registerKeywordGroup(w, req)
		return
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

// registerKeywordGroup registers one position per keyword sharing one budget.
func (h *AdvertiserHandler) registerKeywordGroup(w http.ResponseWriter, req registerRequest) {
	positions, err := h.Registry.RegisterKeywordGroupWithBudget(req.Name, req.Keywords, req.Sigma, req.BidPrice, req.Budget, req.Currency, req.URL)
	if err != nil {
		http.Error(w, "registration failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	head := positions[0]
	h.Budgets.Set(head.ID, req.Budget, head.Currency)

	posList := make([]map[string]interface{}, len(positions))
	for i, pos := range positions {
		posList[i] = map[string]interface{}{
			"id":        pos.ID,
			"name":      pos.Name,
			"keyword":   pos.Intent,
			"sigma":     pos.Sigma,
			"bid_price": pos.BidPrice,
			"currency":  pos.Currency,
			"url":       pos.URL,
			"budget_id": pos.BudgetKey(),
		}
	}

	resp := map[string]interface{}{
		"budget_id": head.ID,
		"positions": posList,
	}
	if h.DB != nil {
		if token, err := h.DB.GenerateToken(head.ID); err == nil {
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

	sigma := -1.0 // negative = keep existing; 0 is a real value (keyword limit)
	if req.Sigma != nil {
		if *req.Sigma < 0 {
			http.Error(w, "sigma must not be negative", http.StatusBadRequest)
			return
		}
		sigma = *req.Sigma
	}

	pos, err := h.Registry.Update(id, req.Name, req.Intent, req.URL, sigma, req.BidPrice)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if req.Budget > 0 && h.DB != nil {
		// Budget lives on the group head; updating a member routes there.
		budgetID := pos.BudgetKey()
		if err := h.DB.UpdateBudget(budgetID, req.Budget); err != nil {
			http.Error(w, "failed to update budget: "+err.Error(), http.StatusInternalServerError)
			return
		}
		h.Budgets.Set(budgetID, req.Budget, pos.Currency)
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
