package handler

import (
	"vectorspace/enclave"
	"vectorspace/platform"
	"vectorspace/tee"
	"encoding/json"
	"net/http"
)

// TEEHandler handles TEE-related HTTP endpoints.
type TEEHandler struct {
	Proxy  tee.TEEProxyInterface
	DB     *platform.DB
	Engine *platform.AuctionEngine
}

// HandleAttestation handles GET /tee/attestation.
// Returns the enclave's public key and attestation document.
func (h *TEEHandler) HandleAttestation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	attest, err := h.Proxy.GetAttestation()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(attest)
}

// HandleAdRequestPrivate handles POST /ad-request.
// Receives an encrypted embedding, forwards to the enclave, returns the result.
func (h *TEEHandler) HandleAdRequestPrivate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req enclave.AuctionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.EncryptedEmbedding.AESKeyEncrypted == "" || req.EncryptedEmbedding.EncryptedPayload == "" {
		http.Error(w, "encrypted_embedding fields are required", http.StatusBadRequest)
		return
	}

	// Look up publisher's log base if not set in request
	if req.LogBase <= 0 && req.PublisherID != "" && h.DB != nil {
		req.LogBase = h.DB.GetPublisherLogBase(req.PublisherID)
	}

	resp, err := h.Proxy.RunAuction(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Log to DB with intent = "[tee-private]"
	var auctionID int64
	if h.DB != nil {
		var logErr error
		if req.PublisherID != "" {
			auctionID, logErr = h.DB.LogAuctionReturningIDWithPublisher("[tee-private]", resp.WinnerID, resp.Payment, resp.Currency, resp.BidCount, req.PublisherID)
		} else {
			auctionID, logErr = h.DB.LogAuctionReturningID("[tee-private]", resp.WinnerID, resp.Payment, resp.Currency, resp.BidCount)
		}
		if logErr != nil {
			h.DB.LogAuction("[tee-private]", resp.WinnerID, resp.Payment, resp.Currency, resp.BidCount)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"auction_id": auctionID,
		"winner_id":  resp.WinnerID,
		"payment":    resp.Payment,
		"currency":   resp.Currency,
		"bid_count":  resp.BidCount,
	})
}
