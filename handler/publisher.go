package handler

import (
	"vectorspace/platform"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

type embedBody struct {
	Text string `json:"text"`
}

type PublisherHandler struct {
	Engine *platform.AuctionEngine
	DB     *platform.DB
}

// HandleRegisterPublisher handles POST /publisher/register
func (h *PublisherHandler) HandleRegisterPublisher(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name   string `json:"name"`
		Domain string `json:"domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	nextID, err := h.DB.NextPublisherID()
	if err != nil {
		http.Error(w, "failed to generate ID: "+err.Error(), http.StatusInternalServerError)
		return
	}
	id := fmt.Sprintf("pub-%d", nextID)

	if err := h.DB.InsertPublisher(id, req.Name, req.Domain); err != nil {
		http.Error(w, "failed to create publisher: "+err.Error(), http.StatusInternalServerError)
		return
	}

	token, err := h.DB.GeneratePublisherToken(id)
	if err != nil {
		http.Error(w, "failed to generate token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"id":     id,
		"name":   req.Name,
		"domain": req.Domain,
		"token":  token,
	})
}

// HandleCreatePublisherWithCredentials handles POST /admin/publishers.
// Creates a publisher with email/password credentials.
func (h *PublisherHandler) HandleCreatePublisherWithCredentials(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name     string `json:"name"`
		Domain   string `json:"domain"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Email == "" || req.Password == "" {
		http.Error(w, "name, email, and password are required", http.StatusBadRequest)
		return
	}

	nextID, err := h.DB.NextPublisherID()
	if err != nil {
		http.Error(w, "failed to generate ID: "+err.Error(), http.StatusInternalServerError)
		return
	}
	id := fmt.Sprintf("pub-%d", nextID)

	if err := h.DB.InsertPublisher(id, req.Name, req.Domain); err != nil {
		http.Error(w, "failed to create publisher: "+err.Error(), http.StatusInternalServerError)
		return
	}

	token, err := h.DB.GeneratePublisherToken(id)
	if err != nil {
		http.Error(w, "failed to generate token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "failed to hash password: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := h.DB.InsertPublisherCredentials(id, req.Email, string(hash)); err != nil {
		http.Error(w, "failed to store credentials: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"id":     id,
		"name":   req.Name,
		"domain": req.Domain,
		"email":  req.Email,
		"token":  token,
	})
}

// HandlePublisherLogin handles POST /publisher/login.
// Authenticates a publisher by email/password and returns their token.
func (h *PublisherHandler) HandlePublisherLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Email == "" || req.Password == "" {
		http.Error(w, "email and password are required", http.StatusBadRequest)
		return
	}

	publisherID, passwordHash, err := h.DB.LookupPublisherByEmail(req.Email)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if publisherID == "" {
		http.Error(w, "invalid email or password", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		http.Error(w, "invalid email or password", http.StatusUnauthorized)
		return
	}

	token, err := h.DB.GetPublisherToken(publisherID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if token == "" {
		http.Error(w, "no token found for publisher", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token":        token,
		"publisher_id": publisherID,
	})
}

// HandleEmbeddings handles GET /embeddings.
// Returns all advertiser embeddings with ETag caching for SDK sync.
func (h *PublisherHandler) HandleEmbeddings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	etag := `"` + h.Engine.Registry.EmbeddingsVersion() + `"`

	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	positions := h.Engine.Registry.GetAll()

	type embeddingEntry struct {
		ID        string    `json:"id"`
		Name      string    `json:"name"`
		Embedding []float64 `json:"embedding"`
		BidPrice  float64   `json:"bid_price"`
		Sigma     float64   `json:"sigma"`
		Currency  string    `json:"currency"`
	}

	entries := make([]embeddingEntry, 0, len(positions))
	for _, p := range positions {
		entries = append(entries, embeddingEntry{
			ID:        p.ID,
			Name:      p.Name,
			Embedding: p.Embedding,
			BidPrice:  p.BidPrice,
			Sigma:     p.Sigma,
			Currency:  p.Currency,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("ETag", etag)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"embeddings": entries,
	})
}

// HandleEmbed handles POST /embed.
// Proxies to the embedding sidecar to embed arbitrary text.
func (h *PublisherHandler) HandleEmbed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req embedBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Text == "" {
		http.Error(w, "text is required", http.StatusBadRequest)
		return
	}

	embedding, err := h.Engine.Embedder.Embed(req.Text)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"embedding": embedding,
	})
}

// HandleSimulate handles POST /simulate.
// Runs a simulated auction with no logging or billing.
func (h *PublisherHandler) HandleSimulate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Intent string  `json:"intent"`
		Tau    float64 `json:"tau"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Intent == "" {
		http.Error(w, "intent is required", http.StatusBadRequest)
		return
	}

	resp, err := h.Engine.SimulateAuction(req.Intent, req.Tau)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

