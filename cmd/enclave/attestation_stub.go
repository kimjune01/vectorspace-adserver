//go:build !nitro

package main

import "vectorspace/enclave"

// generateAttestation is the non-Nitro build's stand-in. Outside a Nitro Enclave
// there is no NSM device to sign a real attestation document, so this returns a
// fixed, clearly-labeled sentinel (and no error, so local dev works). It is NOT
// an attestation — a verifier MUST reject this value. Build the enclave binary
// with `-tags nitro` on a Nitro Enclave to emit real documents (see
// attestation_nitro.go). CI must ensure production ships the nitro build.
const attestationMode = "STUB — UNATTESTED, dev only, NOT for production"

func generateAttestation(km *enclave.KeyManager) (string, error) {
	_ = km
	return "unattested-dev-build-not-for-production", nil
}
