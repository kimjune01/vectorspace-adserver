package handler

import (
	"vectorspace/platform"
	"encoding/json"
	"net/http"
)

type EventHandler struct {
	DB            *platform.DB
	Budgets       *platform.BudgetTracker
	FreqCapMax    int
	FreqCapWindow int
}

type eventRequest struct {
	AuctionID    int64  `json:"auction_id"`
	AdvertiserID string `json:"advertiser_id"`
	UserID       string `json:"user_id"`
	PublisherID  string `json:"publisher_id,omitempty"`
}

// HandleImpression handles POST /event/impression
func (h *EventHandler) HandleImpression(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req eventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.AdvertiserID == "" {
		http.Error(w, "advertiser_id is required", http.StatusBadRequest)
		return
	}

	// Check frequency cap if user_id provided
	if req.UserID != "" {
		ok, err := h.DB.CheckFrequencyCap(req.AdvertiserID, req.UserID, h.FreqCapMax, h.FreqCapWindow)
		if err != nil {
			http.Error(w, "frequency cap check failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Error(w, "frequency cap exceeded", http.StatusTooManyRequests)
			return
		}
	}

	if err := h.logEvent(req.AuctionID, req.AdvertiserID, "impression", req.UserID, req.PublisherID); err != nil {
		http.Error(w, "failed to log event: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Increment frequency cap
	if req.UserID != "" {
		if err := h.DB.IncrementFrequencyCap(req.AdvertiserID, req.UserID, h.FreqCapWindow); err != nil {
			// Log but don't fail the request
			_ = err
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HandleClick handles POST /event/click
// This is where money flows: charge the advertiser on click-through.
func (h *EventHandler) HandleClick(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req eventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.AdvertiserID == "" {
		http.Error(w, "advertiser_id is required", http.StatusBadRequest)
		return
	}

	// Charge on first click only (deduplicate)
	if req.AuctionID > 0 && h.Budgets != nil {
		alreadyClicked, err := h.DB.HasClickEvent(req.AuctionID)
		if err == nil && !alreadyClicked {
			_, payment, err := h.DB.GetAuctionPayment(req.AuctionID)
			if err == nil && payment > 0 {
				h.Budgets.Charge(req.AdvertiserID, payment)
			}
		}
	}

	if err := h.logEvent(req.AuctionID, req.AdvertiserID, "click", req.UserID, req.PublisherID); err != nil {
		http.Error(w, "failed to log event: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HandleViewable handles POST /event/viewable
func (h *EventHandler) HandleViewable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req eventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.AdvertiserID == "" {
		http.Error(w, "advertiser_id is required", http.StatusBadRequest)
		return
	}

	if err := h.logEvent(req.AuctionID, req.AdvertiserID, "viewable", req.UserID, req.PublisherID); err != nil {
		http.Error(w, "failed to log event: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *EventHandler) logEvent(auctionID int64, advertiserID, eventType, userID, publisherID string) error {
	if publisherID != "" {
		return h.DB.LogEventWithPublisher(auctionID, advertiserID, eventType, userID, publisherID)
	}
	return h.DB.LogEvent(auctionID, advertiserID, eventType, userID)
}
