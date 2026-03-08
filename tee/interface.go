package tee

import "vectorspace/enclave"

// TEEProxyInterface abstracts the TEE proxy for both real Nitro Enclave
// and mock (in-process) implementations.
type TEEProxyInterface interface {
	// GetAttestation returns the enclave's public key and attestation document.
	GetAttestation() (*enclave.AttestationResponse, error)

	// RunAuction forwards an auction request to the enclave and returns the result.
	RunAuction(req *enclave.AuctionRequest) (*enclave.AuctionResponse, error)

	// SyncPositions pushes a full position snapshot to the enclave.
	SyncPositions(positions []enclave.PositionSnapshot) error

	// SyncBudgets pushes a full budget snapshot to the enclave.
	SyncBudgets(budgets []enclave.BudgetSnapshot) error
}
