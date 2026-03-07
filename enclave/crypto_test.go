package enclave

import (
	"encoding/json"
	"math"
	"testing"
)

func TestCryptoRoundTrip384D(t *testing.T) {
	km, err := NewKeyManager()
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}

	// Create a 384-dimensional embedding (BGE-small-en-v1.5 size)
	original := make([]float64, 384)
	for i := range original {
		original[i] = float64(i) * 0.001
	}

	plaintext, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Encrypt with public key
	aesKeyEnc, payloadEnc, nonce, err := EncryptHybrid(plaintext, &km.PrivateKey().PublicKey, HashAlgorithmSHA256)
	if err != nil {
		t.Fatalf("EncryptHybrid: %v", err)
	}

	// Decrypt with private key
	decrypted, err := DecryptHybrid(aesKeyEnc, payloadEnc, nonce, km.PrivateKey(), HashAlgorithmSHA256)
	if err != nil {
		t.Fatalf("DecryptHybrid: %v", err)
	}

	var recovered []float64
	if err := json.Unmarshal(decrypted, &recovered); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(recovered) != len(original) {
		t.Fatalf("dimension mismatch: got %d, want %d", len(recovered), len(original))
	}

	for i := range original {
		if math.Abs(recovered[i]-original[i]) > 1e-15 {
			t.Errorf("embedding[%d] = %v, want %v", i, recovered[i], original[i])
		}
	}
}

func TestCryptoRoundTripSHA1(t *testing.T) {
	km, err := NewKeyManager()
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}

	plaintext := []byte(`[0.1, 0.2, 0.3]`)

	aesKeyEnc, payloadEnc, nonce, err := EncryptHybrid(plaintext, &km.PrivateKey().PublicKey, HashAlgorithmSHA1)
	if err != nil {
		t.Fatalf("EncryptHybrid SHA-1: %v", err)
	}

	decrypted, err := DecryptHybrid(aesKeyEnc, payloadEnc, nonce, km.PrivateKey(), HashAlgorithmSHA1)
	if err != nil {
		t.Fatalf("DecryptHybrid SHA-1: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestKeyManagerPEM(t *testing.T) {
	km, err := NewKeyManager()
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}

	pem := km.PublicKeyPEM()
	if pem == "" {
		t.Fatal("expected non-empty PEM")
	}
	if len(pem) < 100 {
		t.Errorf("PEM too short: %d chars", len(pem))
	}
}
