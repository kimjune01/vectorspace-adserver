package tee

import (
	"vectorspace/enclave"
	"crypto/rsa"
	"fmt"
)

// MockTEEProxy runs the auction in-process for local development.
// No EC2 instance or Nitro Enclave needed.
type MockTEEProxy struct {
	keyManager *enclave.KeyManager
	positions  *enclave.PositionStore
	budgets    *enclave.BudgetStore
}

// NewMockTEEProxy creates a mock proxy with an in-process keypair and stores.
func NewMockTEEProxy() (*MockTEEProxy, error) {
	km, err := enclave.NewKeyManager()
	if err != nil {
		return nil, fmt.Errorf("mock tee: %w", err)
	}
	return &MockTEEProxy{
		keyManager: km,
		positions:  enclave.NewPositionStore(),
		budgets:    enclave.NewBudgetStore(),
	}, nil
}

func (m *MockTEEProxy) GetAttestation() (*enclave.AttestationResponse, error) {
	return &enclave.AttestationResponse{
		PublicKey:      m.keyManager.PublicKeyPEM(),
		AttestationB64: "mock-attestation-not-for-production",
	}, nil
}

func (m *MockTEEProxy) RunAuction(req *enclave.AuctionRequest) (*enclave.AuctionResponse, error) {
	return enclave.ProcessPrivateAuction(req, m.keyManager.PrivateKey(), m.positions, m.budgets)
}

func (m *MockTEEProxy) SyncPositions(positions []enclave.PositionSnapshot) error {
	m.positions.ReplaceAll(positions)
	return nil
}

func (m *MockTEEProxy) SyncBudgets(budgets []enclave.BudgetSnapshot) error {
	m.budgets.ReplaceAll(budgets)
	return nil
}

// KeyManagerPublicKey returns the RSA public key for test encryption.
func (m *MockTEEProxy) KeyManagerPublicKey() rsa.PublicKey {
	return m.keyManager.PrivateKey().PublicKey
}
