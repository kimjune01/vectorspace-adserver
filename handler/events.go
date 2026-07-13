package handler

import (
	"vectorspace/platform"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
)

type EventHandler struct {
	DB            *platform.DB
	Budgets       *platform.BudgetTracker
	Registry      *platform.PositionRegistry
	FreqCapMax    int
	FreqCapWindow int

	// settleMu serializes the check-charge-log sequence so two concurrent
	// clicks on the same auction cannot both pass the first-click check.
	settleMu sync.Mutex
}

// settleClick performs the first-click charge for an auction: dedup check,
// budget charge, click log — serialized under settleMu. The advertiser and
// payment come from the auction record, never from request input.
func (h *EventHandler) settleClick(auctionID int64, winnerID, userID, publisherID string) (int, string) {
	h.settleMu.Lock()
	defer h.settleMu.Unlock()

	alreadyClicked, err := h.DB.HasClickEvent(auctionID)
	if err == nil && !alreadyClicked {
		if _, payment, err := h.DB.GetAuctionPayment(auctionID); err == nil && payment > 0 {
			if h.Budgets != nil && !h.Budgets.Charge(h.budgetKey(winnerID), payment) {
				return http.StatusPaymentRequired, "charge failed: budget missing or exhausted"
			}
		}
	}

	if err := h.logEvent(auctionID, winnerID, "click", userID, publisherID); err != nil {
		return http.StatusInternalServerError, "failed to log event: " + err.Error()
	}
	return http.StatusOK, ""
}

// budgetKey resolves a position ID to the ID whose budget it spends from
// (keyword-group members share the group head's budget).
func (h *EventHandler) budgetKey(advertiserID string) string {
	if h.Registry != nil {
		if pos := h.Registry.Get(advertiserID); pos != nil {
			return pos.BudgetKey()
		}
	}
	if h.DB != nil {
		if pos, err := h.DB.GetAdvertiser(advertiserID); err == nil && pos != nil {
			return pos.BudgetKey()
		}
	}
	return advertiserID
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

	// Every click must reference a real auction — an auction_id of zero is
	// not a validation bypass.
	if req.AuctionID <= 0 {
		http.Error(w, "auction_id is required", http.StatusBadRequest)
		return
	}
	winnerID, _, err := h.DB.GetAuctionPayment(req.AuctionID)
	if err != nil {
		http.Error(w, "unknown auction", http.StatusBadRequest)
		return
	}
	if winnerID != req.AdvertiserID {
		http.Error(w, "advertiser_id does not match auction winner", http.StatusBadRequest)
		return
	}

	if code, msg := h.settleClick(req.AuctionID, winnerID, req.UserID, req.PublisherID); code != http.StatusOK {
		http.Error(w, msg, code)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HandleClickRedirect handles GET /click?auction_id=N — the settlement path
// for renderers that only follow a link (e.g. an OpenRTB adm snippet):
// charge on first click, log, then redirect to the winner's URL.
func (h *EventHandler) HandleClickRedirect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	auctionID, err := strconv.ParseInt(r.URL.Query().Get("auction_id"), 10, 64)
	if err != nil || auctionID <= 0 {
		http.Error(w, "auction_id is required", http.StatusBadRequest)
		return
	}

	winnerID, _, err := h.DB.GetAuctionPayment(auctionID)
	if err != nil || winnerID == "" {
		http.Error(w, "unknown auction", http.StatusNotFound)
		return
	}

	if code, msg := h.settleClick(auctionID, winnerID, "", ""); code != http.StatusOK {
		http.Error(w, msg, code)
		return
	}

	if h.Registry != nil {
		if pos := h.Registry.Get(winnerID); pos != nil && pos.URL != "" {
			http.Redirect(w, r, pos.URL, http.StatusFound)
			return
		}
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
