// Package verify is a reference verifier for AWS Nitro Enclave attestation
// documents (COSE_Sign1, signed by the NSM). The relying party (an SDK, or a
// test) uses it to check that a public key was attested by a genuine enclave
// with expected measurements before encrypting to that key. The SDK ports
// mirror this logic.
//
// Trust roots are injectable via Options.Roots: production pins the AWS Nitro
// root certificate; tests supply a local root. This verifier lives outside the
// enclave TCB (it is the checker, not the checked).
//
// Scope: the CBOR tag-18 unwrap, COSE_Sign1 parse, ES384/SHA-384 signature
// check, X.509 chain to the supplied roots, and PCR / nonce / freshness / alg
// checks are implemented and exercised against self-minted documents in
// verify_test.go. Interop against a real captured AWS Nitro document is a
// pending fixture (needs an EC2 Nitro run to produce one). Shapes follow
// RFC 8152 (COSE) and the AWS NSM attestation spec.
package verify

import (
	"crypto/ecdsa"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/fxamacker/cbor/v2"
)

const (
	coseSign1Tag = 18  // CBOR tag wrapping a COSE_Sign1 object
	algES384     = -35 // COSE algorithm id for ECDSA P-384 / SHA-384
	pcrLen       = 48  // SHA-384 PCR length in bytes
	clockSkew    = 2 * time.Minute
)

// Options controls verification. Roots is required.
type Options struct {
	Roots         *x509.CertPool // trust anchors (production: the AWS Nitro root)
	ExpectedPCRs  map[int][]byte // PCR index -> expected value; every entry must match
	ExpectedNonce []byte         // verifier challenge; must equal doc.Nonce byte-for-byte
	RequireNonce  bool           // if set, verification fails unless ExpectedNonce is provided
	Now           time.Time      // reference time for freshness; zero => time.Now()
	MaxAge        time.Duration  // reject if the doc timestamp is older than this; 0 => skip
}

// Verified is the trusted content, returned only after every check passes.
type Verified struct {
	PublicKey []byte // DER SubjectPublicKeyInfo the enclave attested
	PCRs      map[int][]byte
	ModuleID  string
	Timestamp time.Time
	UserData  []byte
	Nonce     []byte
}

// coseSign1 is the COSE_Sign1 array [protected, unprotected, payload, signature].
type coseSign1 struct {
	_           struct{} `cbor:",toarray"`
	Protected   []byte
	Unprotected cbor.RawMessage
	Payload     []byte
	Signature   []byte
}

// attestationDoc is the CBOR payload of the COSE_Sign1 (AWS NSM shape).
type attestationDoc struct {
	ModuleID    string          `cbor:"module_id"`
	Digest      string          `cbor:"digest"`
	Timestamp   uint64          `cbor:"timestamp"` // ms since epoch
	PCRs        map[uint][]byte `cbor:"pcrs"`
	Certificate []byte          `cbor:"certificate"`
	CABundle    [][]byte        `cbor:"cabundle"`
	PublicKey   []byte          `cbor:"public_key"`
	UserData    []byte          `cbor:"user_data"`
	Nonce       []byte          `cbor:"nonce"`
}

// Verify decodes and fully validates a base64 COSE_Sign1 attestation document.
func Verify(docB64 string, opts Options) (*Verified, error) {
	if opts.Roots == nil {
		return nil, errors.New("verify: Options.Roots is required")
	}
	if opts.RequireNonce && len(opts.ExpectedNonce) == 0 {
		return nil, errors.New("verify: RequireNonce set but no ExpectedNonce provided")
	}

	raw, err := base64.StdEncoding.DecodeString(docB64)
	if err != nil {
		return nil, fmt.Errorf("verify: base64: %w", err)
	}

	c, err := decodeCOSESign1(raw)
	if err != nil {
		return nil, err
	}

	// Enforce the signed algorithm declaration (protected header label 1 = ES384).
	if err := requireES384(c.Protected); err != nil {
		return nil, err
	}

	var d attestationDoc
	if err := cbor.Unmarshal(c.Payload, &d); err != nil {
		return nil, fmt.Errorf("verify: payload decode: %w", err)
	}
	if d.Digest != "SHA384" {
		return nil, fmt.Errorf("verify: PCR digest %q (want SHA384)", d.Digest)
	}
	if len(d.Certificate) == 0 {
		return nil, errors.New("verify: missing leaf certificate")
	}

	leaf, err := x509.ParseCertificate(d.Certificate)
	if err != nil {
		return nil, fmt.Errorf("verify: parse leaf: %w", err)
	}

	// 1. Chain the leaf to the supplied roots via the doc's cabundle.
	intermediates := x509.NewCertPool()
	for i, der := range d.CABundle {
		ca, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, fmt.Errorf("verify: parse cabundle[%d]: %w", i, err)
		}
		intermediates.AddCert(ca)
	}
	verifyTime := opts.Now
	if verifyTime.IsZero() {
		verifyTime = time.Now()
	}
	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:         opts.Roots,
		Intermediates: intermediates,
		CurrentTime:   verifyTime,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}); err != nil {
		return nil, fmt.Errorf("verify: certificate chain: %w", err)
	}

	// 2. Verify the COSE_Sign1 signature with the leaf's public key.
	pub, ok := leaf.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("verify: leaf key is not ECDSA")
	}
	if err := verifyCOSESignature(pub, c.Protected, c.Payload, c.Signature); err != nil {
		return nil, err
	}

	// 3. Freshness: reject future-dated (beyond clock skew) and stale documents.
	ts := time.UnixMilli(int64(d.Timestamp))
	if ts.After(verifyTime.Add(clockSkew)) {
		return nil, fmt.Errorf("verify: attestation timestamp is in the future (%s)", ts.Sub(verifyTime))
	}
	if opts.MaxAge > 0 && verifyTime.Sub(ts) > opts.MaxAge {
		return nil, fmt.Errorf("verify: stale attestation (age %s > max %s)", verifyTime.Sub(ts), opts.MaxAge)
	}

	// 4. Nonce: the verifier's challenge must be echoed exactly.
	if len(opts.ExpectedNonce) > 0 {
		if !bytesEqualConst(opts.ExpectedNonce, d.Nonce) {
			return nil, errors.New("verify: nonce mismatch (possible replay)")
		}
	}

	// 5. PCRs: every expected measurement must match, at the SHA-384 length.
	pcrs := make(map[int][]byte, len(d.PCRs))
	for idx, val := range d.PCRs {
		if len(val) != pcrLen {
			return nil, fmt.Errorf("verify: PCR%d length %d (want %d)", idx, len(val), pcrLen)
		}
		pcrs[int(idx)] = val
	}
	for idx, want := range opts.ExpectedPCRs {
		if len(want) != pcrLen {
			return nil, fmt.Errorf("verify: expected PCR%d length %d (want %d)", idx, len(want), pcrLen)
		}
		got, ok := pcrs[idx]
		if !ok {
			return nil, fmt.Errorf("verify: PCR%d absent", idx)
		}
		if !bytesEqualConst(want, got) {
			return nil, fmt.Errorf("verify: PCR%d mismatch", idx)
		}
	}

	// Application policy: this verifier attests a key to encrypt to, so a
	// document without one is rejected (public_key is optional in the NSM format).
	if len(d.PublicKey) == 0 {
		return nil, errors.New("verify: attestation carries no public key")
	}

	return &Verified{
		PublicKey: d.PublicKey,
		PCRs:      pcrs,
		ModuleID:  d.ModuleID,
		Timestamp: ts,
		UserData:  d.UserData,
		Nonce:     d.Nonce,
	}, nil
}

// decodeCOSESign1 unwraps the CBOR tag-18 (COSE_Sign1) that real NSM documents
// carry, then decodes the array. A bare untagged array is also accepted.
func decodeCOSESign1(raw []byte) (*coseSign1, error) {
	var rt cbor.RawTag
	if err := cbor.Unmarshal(raw, &rt); err == nil && rt.Number == coseSign1Tag {
		var c coseSign1
		if err := cbor.Unmarshal(rt.Content, &c); err != nil {
			return nil, fmt.Errorf("verify: COSE_Sign1 decode (tagged): %w", err)
		}
		return &c, nil
	}
	var c coseSign1
	if err := cbor.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("verify: COSE_Sign1 decode: %w", err)
	}
	return &c, nil
}

// requireES384 decodes the protected header and requires alg (label 1) == ES384.
func requireES384(protected []byte) error {
	if len(protected) == 0 {
		return errors.New("verify: empty protected header (no alg)")
	}
	var hdr map[int]interface{}
	if err := cbor.Unmarshal(protected, &hdr); err != nil {
		return fmt.Errorf("verify: protected header decode: %w", err)
	}
	alg, ok := hdr[1]
	if !ok {
		return errors.New("verify: protected header missing alg")
	}
	if v, ok := alg.(int64); !ok || v != algES384 {
		return fmt.Errorf("verify: protected alg %v (want %d ES384)", alg, algES384)
	}
	return nil
}

// verifyCOSESignature checks an ES384 COSE_Sign1 signature. The signature is the
// raw r||s (each 48 bytes for P-384), and the signed bytes are the Sig_structure
// ["Signature1", protected, external_aad(empty), payload] per RFC 8152 §4.4.
func verifyCOSESignature(pub *ecdsa.PublicKey, protected, payload, sig []byte) error {
	if pub.Curve.Params().BitSize != 384 {
		return fmt.Errorf("verify: unexpected curve %s (want P-384)", pub.Curve.Params().Name)
	}
	if len(sig) != 96 {
		return fmt.Errorf("verify: signature length %d (want 96)", len(sig))
	}
	sigStructure := []interface{}{"Signature1", protected, []byte{}, payload}
	toSign, err := cbor.Marshal(sigStructure)
	if err != nil {
		return fmt.Errorf("verify: encode Sig_structure: %w", err)
	}
	digest := sha512.Sum384(toSign)
	r := new(big.Int).SetBytes(sig[:48])
	s := new(big.Int).SetBytes(sig[48:])
	if !ecdsa.Verify(pub, digest[:], r, s) {
		return errors.New("verify: COSE signature invalid")
	}
	return nil
}

// bytesEqualConst is a constant-time equality check.
func bytesEqualConst(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	return v == 0
}
