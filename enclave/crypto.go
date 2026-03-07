package enclave

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"hash"
)

// HashAlgorithm specifies which hash function to use in RSA-OAEP decryption.
type HashAlgorithm string

const (
	HashAlgorithmSHA256 HashAlgorithm = "SHA-256"
	HashAlgorithmSHA1   HashAlgorithm = "SHA-1"
)

// GenerateRSAKeyPair generates a new RSA-2048 key pair.
func GenerateRSAKeyPair() (*rsa.PrivateKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA key pair: %w", err)
	}
	return privateKey, nil
}

func newHash(hashAlg HashAlgorithm) (hash.Hash, error) {
	switch hashAlg {
	case HashAlgorithmSHA256:
		return sha256.New(), nil
	case HashAlgorithmSHA1:
		return sha1.New(), nil
	default:
		return nil, fmt.Errorf("unsupported hash algorithm: %s", hashAlg)
	}
}

// DecryptHybrid decrypts data encrypted with hybrid RSA-OAEP + AES-256-GCM.
func DecryptHybrid(encryptedAESKey, encryptedPayload, nonceB64 string, privateKey *rsa.PrivateKey, hashAlg HashAlgorithm) ([]byte, error) {
	encryptedAESKeyBytes, err := base64.StdEncoding.DecodeString(encryptedAESKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode encrypted AES key: %w", err)
	}

	encryptedPayloadBytes, err := base64.StdEncoding.DecodeString(encryptedPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode encrypted payload: %w", err)
	}

	nonceBytes, err := base64.StdEncoding.DecodeString(nonceB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode nonce: %w", err)
	}

	hasher, err := newHash(hashAlg)
	if err != nil {
		return nil, err
	}

	aesKey, err := rsa.DecryptOAEP(hasher, rand.Reader, privateKey, encryptedAESKeyBytes, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt AES key: %w", err)
	}

	if len(aesKey) != 32 {
		return nil, fmt.Errorf("invalid AES key length: expected 32 bytes, got %d", len(aesKey))
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	if len(nonceBytes) != aesgcm.NonceSize() {
		return nil, fmt.Errorf("invalid nonce length: expected %d bytes, got %d", aesgcm.NonceSize(), len(nonceBytes))
	}

	plaintext, err := aesgcm.Open(nil, nonceBytes, encryptedPayloadBytes, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt payload: %w", err)
	}

	return plaintext, nil
}

// EncryptHybrid encrypts data using hybrid RSA-OAEP + AES-256-GCM.
// Used in tests to verify round-trip encryption/decryption.
func EncryptHybrid(plaintext []byte, publicKey *rsa.PublicKey, hashAlg HashAlgorithm) (aesKeyEncrypted, encryptedPayload, nonceB64 string, err error) {
	// Generate random AES-256 key
	aesKey := make([]byte, 32)
	if _, err = rand.Read(aesKey); err != nil {
		return "", "", "", fmt.Errorf("failed to generate AES key: %w", err)
	}

	// Encrypt AES key with RSA-OAEP
	hasher, err := newHash(hashAlg)
	if err != nil {
		return "", "", "", err
	}
	encAESKey, err := rsa.EncryptOAEP(hasher, rand.Reader, publicKey, aesKey, nil)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to encrypt AES key: %w", err)
	}

	// Encrypt payload with AES-256-GCM
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create AES cipher: %w", err)
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, aesgcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", "", "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := aesgcm.Seal(nil, nonce, plaintext, nil)

	return base64.StdEncoding.EncodeToString(encAESKey),
		base64.StdEncoding.EncodeToString(ciphertext),
		base64.StdEncoding.EncodeToString(nonce),
		nil
}
