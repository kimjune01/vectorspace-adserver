package handler

import (
	"cloudx-adserver/platform"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

type PortalHandler struct {
	Registry *platform.PositionRegistry
	Budgets  *platform.BudgetTracker
	DB       *platform.DB
}

// --- Token-authenticated advertiser endpoints ---

func (h *PortalHandler) authenticateToken(r *http.Request) (string, error) {
	token := r.URL.Query().Get("token")
	if token == "" {
		return "", fmt.Errorf("token is required")
	}
	advertiserID, err := h.DB.LookupToken(token)
	if err != nil {
		return "", fmt.Errorf("token lookup failed: %w", err)
	}
	if advertiserID == "" {
		return "", fmt.Errorf("invalid token")
	}
	return advertiserID, nil
}

// HandlePortalMe handles GET/PUT /portal/me?token=xxx
func (h *PortalHandler) HandlePortalMe(w http.ResponseWriter, r *http.Request) {
	advertiserID, err := h.authenticateToken(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handlePortalMeGet(w, advertiserID)
	case http.MethodPut:
		h.handlePortalMeUpdate(w, r, advertiserID)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *PortalHandler) handlePortalMeGet(w http.ResponseWriter, advertiserID string) {
	pos := h.Registry.Get(advertiserID)
	if pos == nil {
		http.Error(w, "advertiser not found", http.StatusNotFound)
		return
	}

	budget := h.Budgets.GetInfo(advertiserID)

	resp := map[string]interface{}{
		"id":        pos.ID,
		"name":      pos.Name,
		"intent":    pos.Intent,
		"sigma":     pos.Sigma,
		"bid_price": pos.BidPrice,
		"currency":  pos.Currency,
		"url":       pos.URL,
	}
	if budget != nil {
		resp["budget_total"] = budget.Total
		resp["budget_spent"] = budget.Spent
		resp["budget_remaining"] = budget.Remaining
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *PortalHandler) handlePortalMeUpdate(w http.ResponseWriter, r *http.Request, advertiserID string) {
	var req struct {
		Name     string  `json:"name"`
		Intent   string  `json:"intent"`
		Sigma    float64 `json:"sigma"`
		BidPrice float64 `json:"bid_price"`
		Budget   float64 `json:"budget"`
		URL      string  `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	pos, err := h.Registry.Update(advertiserID, req.Name, req.Intent, req.URL, req.Sigma, req.BidPrice)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if req.Budget > 0 && h.DB != nil {
		if err := h.DB.UpdateBudget(advertiserID, req.Budget); err != nil {
			http.Error(w, "failed to update budget: "+err.Error(), http.StatusInternalServerError)
			return
		}
		h.Budgets.Set(advertiserID, req.Budget, pos.Currency)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pos)
}

// HandlePortalAuctions handles GET /portal/me/auctions?token=xxx&limit=&offset=&format=csv
func (h *PortalHandler) HandlePortalAuctions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	advertiserID, err := h.authenticateToken(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	format := r.URL.Query().Get("format")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if format == "csv" {
		limit = 0
		offset = 0
	}

	auctions, total, err := h.DB.GetAuctionsByAdvertiser(advertiserID, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=my-auctions.csv")
		cw := csv.NewWriter(w)
		cw.Write([]string{"id", "intent", "payment", "currency", "bid_count", "created_at"})
		for _, a := range auctions {
			cw.Write([]string{
				strconv.FormatInt(a.ID, 10),
				a.Intent,
				fmt.Sprintf("%.4f", a.Payment),
				a.Currency,
				strconv.Itoa(a.BidCount),
				a.CreatedAt,
			})
		}
		cw.Flush()
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"auctions": auctions,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

// HandlePortalEvents handles GET /portal/me/events?token=xxx
func (h *PortalHandler) HandlePortalEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	advertiserID, err := h.authenticateToken(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	stats, err := h.DB.GetEventStats(advertiserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// --- Admin endpoints ---

// HandleAdminAuctions handles GET /admin/auctions?limit=&offset=&winner=&intent=&format=csv
func (h *PortalHandler) HandleAdminAuctions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	winner := r.URL.Query().Get("winner")
	intent := r.URL.Query().Get("intent")
	format := r.URL.Query().Get("format")

	auctions, total, err := h.DB.GetAllAuctions(limit, offset, winner, intent)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=auctions.csv")
		cw := csv.NewWriter(w)
		cw.Write([]string{"id", "intent", "winner_id", "winner_name", "payment", "currency", "bid_count", "created_at"})
		for _, a := range auctions {
			cw.Write([]string{
				strconv.FormatInt(a.ID, 10),
				a.Intent,
				a.WinnerID,
				a.WinnerName,
				fmt.Sprintf("%.4f", a.Payment),
				a.Currency,
				strconv.Itoa(a.BidCount),
				a.CreatedAt,
			})
		}
		cw.Flush()
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"auctions": auctions,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

// HandleAdminRevenue handles GET /admin/revenue?group_by=day|week|month
func (h *PortalHandler) HandleAdminRevenue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	groupBy := r.URL.Query().Get("group_by")
	if groupBy == "" {
		groupBy = "day"
	}

	periods, err := h.DB.GetRevenueByPeriod(groupBy)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"group_by": groupBy,
		"periods":  periods,
	})
}

// HandleAdminTopAdvertisers handles GET /admin/top-advertisers?limit=
func (h *PortalHandler) HandleAdminTopAdvertisers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	advertisers, err := h.DB.GetTopAdvertisersBySpend(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(advertisers)
}

// HandleAdminAdvertisers handles GET /admin/advertisers
func (h *PortalHandler) HandleAdminAdvertisers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	advertisers, err := h.DB.GetAllAdvertisersWithBudget()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(advertisers)
}

// HandleAdminEvents handles GET /admin/events
func (h *PortalHandler) HandleAdminEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats, err := h.DB.GetEventStats("")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// HandleAdminPublishers handles GET /admin/publishers
func (h *PortalHandler) HandleAdminPublishers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	publishers, err := h.DB.GetAllPublishers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(publishers)
}

// --- Publisher Portal ---

type PublisherPortalHandler struct {
	DB *platform.DB
}

func (h *PublisherPortalHandler) authenticatePublisherToken(r *http.Request) (string, error) {
	token := r.URL.Query().Get("token")
	if token == "" {
		return "", fmt.Errorf("token is required")
	}
	publisherID, err := h.DB.LookupPublisherToken(token)
	if err != nil {
		return "", fmt.Errorf("token lookup failed: %w", err)
	}
	if publisherID == "" {
		return "", fmt.Errorf("invalid token")
	}
	return publisherID, nil
}

// HandlePublisherMe handles GET /portal/publisher/me?token=
func (h *PublisherPortalHandler) HandlePublisherMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	publisherID, err := h.authenticatePublisherToken(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	pub, err := h.DB.GetPublisher(publisherID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if pub == nil {
		http.Error(w, "publisher not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pub)
}

// HandlePublisherStats handles GET /portal/publisher/stats?token=
func (h *PublisherPortalHandler) HandlePublisherStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	publisherID, err := h.authenticatePublisherToken(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	stats, err := h.DB.GetPublisherStats(publisherID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// HandlePublisherRevenue handles GET /portal/publisher/revenue?token=&group_by=
func (h *PublisherPortalHandler) HandlePublisherRevenue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	publisherID, err := h.authenticatePublisherToken(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	groupBy := r.URL.Query().Get("group_by")
	if groupBy == "" {
		groupBy = "day"
	}

	periods, err := h.DB.GetPublisherRevenueByPeriod(publisherID, groupBy)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"group_by": groupBy,
		"periods":  periods,
	})
}

// HandlePublisherEvents handles GET /portal/publisher/events?token=
func (h *PublisherPortalHandler) HandlePublisherEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	publisherID, err := h.authenticatePublisherToken(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	stats, err := h.DB.GetPublisherEventStats(publisherID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// HandlePublisherAuctions handles GET /portal/publisher/auctions?token=&limit=&offset=
func (h *PublisherPortalHandler) HandlePublisherAuctions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	publisherID, err := h.authenticatePublisherToken(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	auctions, total, err := h.DB.GetAuctionsByPublisher(publisherID, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"auctions": auctions,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

// HandlePublisherTopAdvertisers handles GET /portal/publisher/top-advertisers?token=&limit=
func (h *PublisherPortalHandler) HandlePublisherTopAdvertisers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	publisherID, err := h.authenticatePublisherToken(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	advertisers, err := h.DB.GetPublisherTopAdvertisers(publisherID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(advertisers)
}
