package enclave

// EncryptedEmbedding is the hybrid-encrypted embedding from the SDK.
type EncryptedEmbedding struct {
	AESKeyEncrypted  string `json:"aes_key_encrypted"`
	EncryptedPayload string `json:"encrypted_payload"`
	Nonce            string `json:"nonce"`
	HashAlgorithm    string `json:"hash_algorithm"`
}

// AuctionRequest is sent by the parent to the enclave.
type AuctionRequest struct {
	EncryptedEmbedding EncryptedEmbedding `json:"encrypted_embedding"`
	Tau                float64            `json:"tau,omitempty"`
	PublisherID        string             `json:"publisher_id,omitempty"`
}

// AuctionResponse is returned from the enclave to the parent.
type AuctionResponse struct {
	WinnerID string  `json:"winner_id"`
	Payment  float64 `json:"payment"`
	Currency string  `json:"currency"`
	BidCount int     `json:"bid_count"`
}

// PositionSnapshot is an advertiser position pushed from parent to enclave.
type PositionSnapshot struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Embedding []float64 `json:"embedding"`
	Sigma     float64   `json:"sigma"`
	BidPrice  float64   `json:"bid_price"`
	Currency  string    `json:"currency"`
	URL       string    `json:"url"`
}

// BudgetSnapshot is a budget record pushed from parent to enclave.
type BudgetSnapshot struct {
	AdvertiserID string  `json:"advertiser_id"`
	Total        float64 `json:"total"`
	Spent        float64 `json:"spent"`
	Currency     string  `json:"currency"`
}

// AttestationResponse is returned by the key_request message.
type AttestationResponse struct {
	PublicKey     string `json:"public_key"`
	AttestationB64 string `json:"attestation_cose_base64"`
}
