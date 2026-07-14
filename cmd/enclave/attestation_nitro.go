//go:build nitro

package main

import (
	"vectorspace/enclave"
	"crypto/rand"
	"encoding/base64"
	"encoding/pem"
	"fmt"

	"github.com/hf/nsm"
	"github.com/hf/nsm/request"
)

// generateAttestation asks the Nitro Security Module for a COSE_Sign1
// attestation document that binds the enclave's public key and a fresh nonce to
// the enclave's PCR measurements. A verifier checks the document's signature
// against the AWS Nitro root and compares PCRs before trusting the embedded key
// (see the SDK-side verifier). Returns the base64-encoded document.
//
// Fails closed: on any error it returns a non-nil error, and the caller must
// fail the key_request rather than hand back an unattested key.
//
// UserData carries a domain-separation tag so a valid document cannot be
// repurposed across protocols. Verifier-supplied challenge freshness (the SDK
// choosing the Nonce and checking it byte-for-byte) lands with the SDK verifier
// in the next checkpoint; today the Nonce is enclave-generated, which binds the
// key but does not yet prove liveness to a remote verifier.
const attestationMode = "nitro (real NSM attestation)"

func generateAttestation(km *enclave.KeyManager) (string, error) {
	doc, err := attestationDocument(km)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(doc), nil
}

func attestationDocument(km *enclave.KeyManager) ([]byte, error) {
	block, _ := pem.Decode([]byte(km.PublicKeyPEM()))
	if block == nil {
		return nil, fmt.Errorf("decode public key PEM")
	}

	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	sess, err := nsm.OpenDefaultSession()
	if err != nil {
		return nil, fmt.Errorf("open NSM session: %w", err)
	}
	defer sess.Close()

	res, err := sess.Send(&request.Attestation{
		PublicKey: block.Bytes, // DER SubjectPublicKeyInfo
		Nonce:     nonce,
		UserData:  []byte("vectorspace/key-attestation/v1"), // domain separation
	})
	if err != nil {
		return nil, fmt.Errorf("NSM attestation request: %w", err)
	}
	if res.Error != "" {
		return nil, fmt.Errorf("NSM error: %s", res.Error)
	}
	if res.Attestation == nil || len(res.Attestation.Document) == 0 {
		return nil, fmt.Errorf("NSM returned empty attestation document")
	}
	return res.Attestation.Document, nil
}
