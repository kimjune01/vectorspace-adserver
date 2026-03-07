package enclave

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"sync"
)

// KeyManager holds the RSA keypair and exports the public key as PEM.
type KeyManager struct {
	mu         sync.RWMutex
	privateKey *rsa.PrivateKey
	publicPEM  string
}

// NewKeyManager generates a fresh RSA-2048 keypair.
func NewKeyManager() (*KeyManager, error) {
	privKey, err := GenerateRSAKeyPair()
	if err != nil {
		return nil, err
	}

	pubDER, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %w", err)
	}

	pubPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER,
	}))

	return &KeyManager{
		privateKey: privKey,
		publicPEM:  pubPEM,
	}, nil
}

// PublicKeyPEM returns the PEM-encoded public key.
func (km *KeyManager) PublicKeyPEM() string {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return km.publicPEM
}

// PrivateKey returns the RSA private key (used for decryption).
func (km *KeyManager) PrivateKey() *rsa.PrivateKey {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return km.privateKey
}
