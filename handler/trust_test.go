package handler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"vectorspace/trust"

	_ "modernc.org/sqlite"
)

func setupTrustHandler(t *testing.T) (*TrustHandler, *trust.Ledger) {
	t.Helper()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	ledger, err := trust.NewLedger(conn)
	if err != nil {
		t.Fatalf("new ledger: %v", err)
	}

	return &TrustHandler{Ledger: ledger}, ledger
}

func TestHTTPAttestationFlow(t *testing.T) {
	h, _ := setupTrustHandler(t)

	// Submit attestation via HTTP
	body := map[string]any{
		"sender_domain":    "stripe.com",
		"attestation_id":   "http_stripe_test",
		"attestation_type": "payment_processor",
		"subject":          "merchant@example.com",
		"duration_years":   3,
		"status":           "good_standing",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/trust/attest", bytes.NewReader(b))
	w := httptest.NewRecorder()
	h.HandleSubmitAttestation(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "pending" {
		t.Errorf("expected pending, got %s", resp["status"])
	}
	if resp["edge_kind"] != "bilateral" {
		t.Errorf("expected bilateral, got %s", resp["edge_kind"])
	}

	// Confirm via HTTP
	confirmBody := map[string]any{
		"sender_domain":  "example.com",
		"action":         "confirm",
		"attestation_id": "http_stripe_test",
	}
	b, _ = json.Marshal(confirmBody)
	req = httptest.NewRequest("POST", "/trust/attest", bytes.NewReader(b))
	w = httptest.NewRecorder()
	h.HandleSubmitAttestation(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("confirm: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Check trust graph has edges
	req = httptest.NewRequest("GET", "/trust/graph", nil)
	w = httptest.NewRecorder()
	h.HandleGraph(w, req)

	var graphResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &graphResp)
	edgeCount := graphResp["count"].(float64)
	if edgeCount != 2 {
		t.Errorf("expected 2 edges in graph, got %v", edgeCount)
	}

	// Check node info
	req = httptest.NewRequest("GET", "/trust/node/example.com", nil)
	w = httptest.NewRecorder()
	h.HandleNode(w, req)

	var nodeResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &nodeResp)
	node := nodeResp["node"].(map[string]any)
	if node["bilateral_count"].(float64) != 2 {
		t.Errorf("expected 2 bilateral edges for node, got %v", node["bilateral_count"])
	}
}

func TestHTTPUnilateralAttestation(t *testing.T) {
	h, _ := setupTrustHandler(t)

	body := map[string]any{
		"sender_domain":    "google.com",
		"attestation_id":   "google_rating_1",
		"attestation_type": "platform_rating",
		"subject":          "restaurant@example.com",
		"rating":           4.5,
		"review_count":     247,
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/trust/attest", bytes.NewReader(b))
	w := httptest.NewRecorder()
	h.HandleSubmitAttestation(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "confirmed" {
		t.Errorf("expected confirmed (unilateral), got %s", resp["status"])
	}
	if resp["edge_kind"] != "unilateral" {
		t.Errorf("expected unilateral, got %s", resp["edge_kind"])
	}

	// Edge should exist immediately
	req = httptest.NewRequest("GET", "/trust/graph", nil)
	w = httptest.NewRecorder()
	h.HandleGraph(w, req)

	var graphResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &graphResp)
	if graphResp["count"].(float64) != 1 {
		t.Errorf("expected 1 edge, got %v", graphResp["count"])
	}
}

func TestHTTPRevocation(t *testing.T) {
	h, _ := setupTrustHandler(t)

	// Create and confirm
	body := map[string]any{
		"sender_domain":    "stripe.com",
		"attestation_id":   "revoke_test",
		"attestation_type": "payment_processor",
		"subject":          "merchant@example.com",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/trust/attest", bytes.NewReader(b))
	w := httptest.NewRecorder()
	h.HandleSubmitAttestation(w, req)

	confirmBody := map[string]any{
		"sender_domain":  "example.com",
		"action":         "confirm",
		"attestation_id": "revoke_test",
	}
	b, _ = json.Marshal(confirmBody)
	req = httptest.NewRequest("POST", "/trust/attest", bytes.NewReader(b))
	w = httptest.NewRecorder()
	h.HandleSubmitAttestation(w, req)

	// Revoke
	revokeBody := map[string]any{
		"sender_domain":  "stripe.com",
		"action":         "revoke",
		"attestation_id": "revoke_test",
		"reason":         "account_closed",
	}
	b, _ = json.Marshal(revokeBody)
	req = httptest.NewRequest("POST", "/trust/attest", bytes.NewReader(b))
	w = httptest.NewRecorder()
	h.HandleSubmitAttestation(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("revoke: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Graph should be empty
	req = httptest.NewRequest("GET", "/trust/graph", nil)
	w = httptest.NewRecorder()
	h.HandleGraph(w, req)

	var graphResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &graphResp)
	if graphResp["count"].(float64) != 0 {
		t.Errorf("expected 0 edges after revocation, got %v", graphResp["count"])
	}
}

func TestHTTPAllowlist(t *testing.T) {
	h, _ := setupTrustHandler(t)

	// Build up a rich topology for example.com
	attestations := []map[string]any{
		{"sender_domain": "stripe.com", "attestation_id": "a1", "attestation_type": "payment_processor", "subject": "merchant@example.com"},
		{"sender_domain": "supplier.com", "attestation_id": "a2", "attestation_type": "vendor_relationship", "subject": "merchant@example.com"},
		{"sender_domain": "google.com", "attestation_id": "a3", "attestation_type": "platform_rating", "subject": "merchant@example.com", "review_count": float64(100)},
	}
	for _, att := range attestations {
		b, _ := json.Marshal(att)
		req := httptest.NewRequest("POST", "/trust/attest", bytes.NewReader(b))
		w := httptest.NewRecorder()
		h.HandleSubmitAttestation(w, req)
	}

	// Confirm bilateral ones
	for _, id := range []string{"a1", "a2"} {
		b, _ := json.Marshal(map[string]any{
			"sender_domain":  "example.com",
			"action":         "confirm",
			"attestation_id": id,
		})
		req := httptest.NewRequest("POST", "/trust/attest", bytes.NewReader(b))
		w := httptest.NewRecorder()
		h.HandleSubmitAttestation(w, req)
	}

	// Query allowlist: min 3 edges, min 1 bilateral
	req := httptest.NewRequest("GET", "/trust/allowlist?min_edges=3&min_bilateral=1", nil)
	w := httptest.NewRecorder()
	h.HandleTrustedDomains(w, req)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	count := resp["count"].(float64)
	if count == 0 {
		t.Error("expected at least one domain in allowlist")
	}
}

func TestHTTPLedgerLog(t *testing.T) {
	h, _ := setupTrustHandler(t)

	// Create an attestation
	body := map[string]any{
		"sender_domain":    "stripe.com",
		"attestation_id":   "log_test",
		"attestation_type": "payment_processor",
		"subject":          "m@example.com",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/trust/attest", bytes.NewReader(b))
	w := httptest.NewRecorder()
	h.HandleSubmitAttestation(w, req)

	// Check log
	req = httptest.NewRequest("GET", "/trust/log?limit=10", nil)
	w = httptest.NewRecorder()
	h.HandleLedgerLog(w, req)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["count"].(float64) != 1 {
		t.Errorf("expected 1 log entry, got %v", resp["count"])
	}
}

func TestHTTPGetAttestation(t *testing.T) {
	h, _ := setupTrustHandler(t)

	body := map[string]any{
		"sender_domain":    "stripe.com",
		"attestation_id":   "get_test",
		"attestation_type": "payment_processor",
		"subject":          "m@example.com",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/trust/attest", bytes.NewReader(b))
	w := httptest.NewRecorder()
	h.HandleSubmitAttestation(w, req)

	// Get it back
	req = httptest.NewRequest("GET", "/trust/attestation/get_test", nil)
	w = httptest.NewRecorder()
	h.HandleAttestation(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["attestation_id"] != "get_test" {
		t.Errorf("expected get_test, got %v", resp["attestation_id"])
	}

	// 404 for nonexistent
	req = httptest.NewRequest("GET", "/trust/attestation/nonexistent", nil)
	w = httptest.NewRecorder()
	h.HandleAttestation(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
