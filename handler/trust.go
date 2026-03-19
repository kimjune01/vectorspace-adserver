package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"vectorspace/trust"
)

// TrustHandler serves the public trust graph ledger over HTTPS.
type TrustHandler struct {
	Ledger *trust.Ledger
}

// HandleGraph serves GET /trust/graph — the full public trust graph.
// This is the append-only feed curators sync.
func (h *TrustHandler) HandleGraph(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	edges, err := h.Ledger.GetGraph()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"edges": edges,
		"count": len(edges),
	})
}

// HandleNode serves GET /trust/node/{domain} — trust info for a single domain.
func (h *TrustHandler) HandleNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	domain := strings.TrimPrefix(r.URL.Path, "/trust/node/")
	if domain == "" {
		http.Error(w, "domain required", http.StatusBadRequest)
		return
	}

	node, err := h.Ledger.GetNode(domain)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	edges, err := h.Ledger.GetEdgesForDomain(domain)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"node":  node,
		"edges": edges,
	})
}

// HandleAttestation serves GET /trust/attestation/{id} — a single attestation.
func (h *TrustHandler) HandleAttestation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/trust/attestation/")
	if id == "" {
		http.Error(w, "attestation_id required", http.StatusBadRequest)
		return
	}

	a, err := h.Ledger.GetAttestation(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if a == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(a)
}

// HandleLedgerLog serves GET /trust/log — the append-only ledger log.
// Query param: ?limit=N (default 100)
func (h *TrustHandler) HandleLedgerLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	entries, err := h.Ledger.GetLedgerLog(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"log":   entries,
		"count": len(entries),
	})
}

// HandleSubmitAttestation serves POST /trust/attest — HTTP-based attestation submission.
// This is the alternative to SMTP for testing and API integration.
// In production, attestations arrive via DKIM-signed email. This endpoint
// simulates the same flow for development without a mail server.
func (h *TrustHandler) HandleSubmitAttestation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// For HTTP submissions, the sender domain comes from the payload
	senderDomain, _ := payload["sender_domain"].(string)
	if senderDomain == "" {
		http.Error(w, "sender_domain is required", http.StatusBadRequest)
		return
	}

	action, _ := payload["action"].(string)

	switch action {
	case "confirm":
		attestationID, _ := payload["attestation_id"].(string)
		if attestationID == "" {
			http.Error(w, "attestation_id required", http.StatusBadRequest)
			return
		}
		if err := h.Ledger.ConfirmAttestation(attestationID, senderDomain); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "confirmed", "attestation_id": attestationID})

	case "revoke":
		attestationID, _ := payload["attestation_id"].(string)
		if attestationID == "" {
			http.Error(w, "attestation_id required", http.StatusBadRequest)
			return
		}
		reason, _ := payload["reason"].(string)
		if err := h.Ledger.RevokeAttestation(attestationID, senderDomain, reason); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "revoked", "attestation_id": attestationID})

	default:
		// New attestation
		attestationID, _ := payload["attestation_id"].(string)
		attestationType, _ := payload["attestation_type"].(string)
		subject, _ := payload["subject"].(string)

		if attestationID == "" || attestationType == "" || subject == "" {
			http.Error(w, "attestation_id, attestation_type, and subject are required", http.StatusBadRequest)
			return
		}

		edgeKind := trust.EdgeBilateral
		status := trust.StatusPending
		if attestationType == trust.TypePlatformRating {
			edgeKind = trust.EdgeUnilateral
			status = trust.StatusConfirmed
		}

		a := &trust.Attestation{
			ID:              attestationID,
			Type:            attestationType,
			AttestorDomain:  senderDomain,
			SubjectEmail:    subject,
			Status:          status,
			EdgeKind:        edgeKind,
			DKIMVerified:    false, // HTTP submissions are not DKIM-verified
			Payload:         payload,
			PublishedFields: payload,
			ReceivedAt:      time.Now(),
		}

		if err := h.Ledger.RecordAttestation(a); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"status":         string(a.Status),
			"attestation_id": a.ID,
			"edge_kind":      string(a.EdgeKind),
		})
	}
}

// HandleTrustedDomains serves GET /trust/allowlist — domains meeting trust thresholds.
// Query params: ?min_edges=N&min_bilateral=N
func (h *TrustHandler) HandleTrustedDomains(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	minEdges := 1
	minBilateral := 0
	if v := r.URL.Query().Get("min_edges"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			minEdges = n
		}
	}
	if v := r.URL.Query().Get("min_bilateral"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			minBilateral = n
		}
	}

	nodes, err := h.Ledger.GetTrustedDomains(minEdges, minBilateral)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"domains": nodes,
		"count":   len(nodes),
		"criteria": map[string]int{
			"min_edges":     minEdges,
			"min_bilateral": minBilateral,
		},
	})
}

// HandlePublishPreference serves PUT /trust/publish — set field publish preferences.
func (h *TrustHandler) HandlePublishPreference(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var pref trust.PublishPreference
	if err := json.NewDecoder(r.Body).Decode(&pref); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if pref.SubjectEmail == "" {
		http.Error(w, "subject_email required", http.StatusBadRequest)
		return
	}

	if err := h.Ledger.SetPublishPreference(&pref); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
