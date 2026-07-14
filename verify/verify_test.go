package verify

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
)

// testPKI is a self-minted root -> intermediate -> leaf chain (all ECDSA P-384),
// standing in for the AWS Nitro PKI so the verifier logic runs fully offline.
type testPKI struct {
	roots          *x509.CertPool
	leafKey        *ecdsa.PrivateKey
	leafDER        []byte
	interDER       []byte
	attestedPubDER []byte // real RSA SPKI the enclave "attests"
}

func p384(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	return k
}

func mintPKI(t *testing.T) testPKI {
	t.Helper()
	rootKey := p384(t)
	interKey := p384(t)
	leafKey := p384(t)

	notBefore := time.Now().Add(-time.Hour)
	notAfter := time.Now().Add(24 * time.Hour)

	rootTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-nitro-root"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTmpl, rootTmpl, &rootKey.PublicKey, rootKey)
	if err != nil {
		t.Fatalf("root cert: %v", err)
	}
	rootCert, _ := x509.ParseCertificate(rootDER)

	interTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "test-nitro-intermediate"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	interDER, err := x509.CreateCertificate(rand.Reader, interTmpl, rootCert, &interKey.PublicKey, rootKey)
	if err != nil {
		t.Fatalf("intermediate cert: %v", err)
	}
	interCert, _ := x509.ParseCertificate(interDER)

	leafTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(3),
		Subject:               pkix.Name{CommonName: "test-nitro-enclave"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature,
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, interCert, &leafKey.PublicKey, interKey)
	if err != nil {
		t.Fatalf("leaf cert: %v", err)
	}

	roots := x509.NewCertPool()
	roots.AddCert(rootCert)

	encKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("enc keygen: %v", err)
	}
	attestedPubDER, err := x509.MarshalPKIXPublicKey(&encKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal attested key: %v", err)
	}

	return testPKI{roots: roots, leafKey: leafKey, leafDER: leafDER, interDER: interDER, attestedPubDER: attestedPubDER}
}

func rawSig(r, s *big.Int) []byte {
	out := make([]byte, 96)
	r.FillBytes(out[:48])
	s.FillBytes(out[48:])
	return out
}

// signDocWith builds a base64 COSE_Sign1 over d, signed by signKey (normally the
// leaf key; a different key produces an invalid COSE signature).
func signDocWith(t *testing.T, pki testPKI, d attestationDoc, signKey *ecdsa.PrivateKey) string {
	t.Helper()
	payload, err := cbor.Marshal(d)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	protected, err := cbor.Marshal(map[int]int{1: -35}) // COSE header: alg = ES384
	if err != nil {
		t.Fatalf("marshal protected: %v", err)
	}
	sigStructure := []interface{}{"Signature1", protected, []byte{}, payload}
	toSign, err := cbor.Marshal(sigStructure)
	if err != nil {
		t.Fatalf("marshal sig structure: %v", err)
	}
	digest := sha512.Sum384(toSign)
	r, s, err := ecdsa.Sign(rand.Reader, signKey, digest[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	unprotected, _ := cbor.Marshal(map[int]int{})
	c := coseSign1{Protected: protected, Unprotected: unprotected, Payload: payload, Signature: rawSig(r, s)}
	// Real NSM documents wrap the COSE_Sign1 in CBOR tag 18.
	raw, err := cbor.Marshal(cbor.Tag{Number: 18, Content: c})
	if err != nil {
		t.Fatalf("marshal cose: %v", err)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

func (pki testPKI) doc(nonce, pcr0 []byte, ts time.Time) attestationDoc {
	return attestationDoc{
		ModuleID:    "i-test-enc",
		Digest:      "SHA384",
		Timestamp:   uint64(ts.UnixMilli()),
		PCRs:        map[uint][]byte{0: pcr0},
		Certificate: pki.leafDER,
		CABundle:    [][]byte{pki.interDER},
		PublicKey:   pki.attestedPubDER,
		UserData:    []byte("vectorspace/key-attestation/v1"),
		Nonce:       nonce,
	}
}

func baseOpts(pki testPKI, nonce, pcr0 []byte) Options {
	return Options{
		Roots:            pki.roots,
		ExpectedPCRs:     map[int][]byte{0: pcr0},
		ExpectedNonce:    nonce,
		ExpectedUserData: []byte("vectorspace/key-attestation/v1"),
		Now:              time.Now(),
		MaxAge:           5 * time.Minute,
	}
}

func TestVerify_valid(t *testing.T) {
	pki := mintPKI(t)
	nonce := []byte("verifier-challenge-32-bytes-xxxx")
	pcr0 := make([]byte, 48)
	docB64 := signDocWith(t, pki, pki.doc(nonce, pcr0, time.Now()), pki.leafKey)

	v, err := Verify(docB64, baseOpts(pki, nonce, pcr0))
	if err != nil {
		t.Fatalf("valid doc rejected: %v", err)
	}
	if _, err := x509.ParsePKIXPublicKey(v.PublicKey); err != nil {
		t.Fatalf("attested key is not a valid SPKI: %v", err)
	}
	if string(v.UserData) != "vectorspace/key-attestation/v1" {
		t.Fatalf("wrong user data: %q", v.UserData)
	}
}

func TestVerify_badSignature(t *testing.T) {
	pki := mintPKI(t)
	nonce := []byte("verifier-challenge-32-bytes-xxxx")
	pcr0 := make([]byte, 48)
	// Sign with a different key than the leaf cert in the doc.
	wrongKey := p384(t)
	docB64 := signDocWith(t, pki, pki.doc(nonce, pcr0, time.Now()), wrongKey)

	if _, err := Verify(docB64, baseOpts(pki, nonce, pcr0)); err == nil {
		t.Fatal("bad signature accepted")
	}
}

func TestVerify_wrongRoot(t *testing.T) {
	pki := mintPKI(t)
	other := mintPKI(t)
	nonce := []byte("verifier-challenge-32-bytes-xxxx")
	pcr0 := make([]byte, 48)
	docB64 := signDocWith(t, pki, pki.doc(nonce, pcr0, time.Now()), pki.leafKey)

	opts := baseOpts(pki, nonce, pcr0)
	opts.Roots = other.roots // trust a different root
	if _, err := Verify(docB64, opts); err == nil {
		t.Fatal("doc chaining to untrusted root accepted")
	}
}

func TestVerify_stale(t *testing.T) {
	pki := mintPKI(t)
	nonce := []byte("verifier-challenge-32-bytes-xxxx")
	pcr0 := make([]byte, 48)
	old := time.Now().Add(-10 * time.Minute)
	docB64 := signDocWith(t, pki, pki.doc(nonce, pcr0, old), pki.leafKey)

	if _, err := Verify(docB64, baseOpts(pki, nonce, pcr0)); err == nil {
		t.Fatal("stale doc accepted")
	}
}

func TestVerify_nonceMismatch(t *testing.T) {
	pki := mintPKI(t)
	nonce := []byte("verifier-challenge-32-bytes-xxxx")
	pcr0 := make([]byte, 48)
	docB64 := signDocWith(t, pki, pki.doc(nonce, pcr0, time.Now()), pki.leafKey)

	opts := baseOpts(pki, nonce, pcr0)
	opts.ExpectedNonce = []byte("a-different-challenge-value-yyyy")
	if _, err := Verify(docB64, opts); err == nil {
		t.Fatal("nonce mismatch accepted (replay)")
	}
}

func TestVerify_pcrMismatch(t *testing.T) {
	pki := mintPKI(t)
	nonce := []byte("verifier-challenge-32-bytes-xxxx")
	pcr0 := make([]byte, 48)
	docB64 := signDocWith(t, pki, pki.doc(nonce, pcr0, time.Now()), pki.leafKey)

	opts := baseOpts(pki, nonce, pcr0)
	want := make([]byte, 48)
	want[0] = 0xff // expected measurement the doc does not carry
	opts.ExpectedPCRs = map[int][]byte{0: want}
	if _, err := Verify(docB64, opts); err == nil {
		t.Fatal("PCR mismatch accepted")
	}
}

func TestVerify_futureDated(t *testing.T) {
	pki := mintPKI(t)
	nonce := []byte("verifier-challenge-32-bytes-xxxx")
	pcr0 := make([]byte, 48)
	future := time.Now().Add(10 * time.Minute)
	docB64 := signDocWith(t, pki, pki.doc(nonce, pcr0, future), pki.leafKey)

	if _, err := Verify(docB64, baseOpts(pki, nonce, pcr0)); err == nil {
		t.Fatal("future-dated attestation accepted")
	}
}

func TestVerify_requireNonceMissing(t *testing.T) {
	pki := mintPKI(t)
	nonce := []byte("verifier-challenge-32-bytes-xxxx")
	pcr0 := make([]byte, 48)
	docB64 := signDocWith(t, pki, pki.doc(nonce, pcr0, time.Now()), pki.leafKey)

	opts := baseOpts(pki, nonce, pcr0)
	opts.ExpectedNonce = nil // nonce required by default
	if _, err := Verify(docB64, opts); err == nil {
		t.Fatal("missing ExpectedNonce accepted without InsecureSkipNonce")
	}
}

func TestVerify_skipNonce(t *testing.T) {
	pki := mintPKI(t)
	nonce := []byte("verifier-challenge-32-bytes-xxxx")
	pcr0 := make([]byte, 48)
	docB64 := signDocWith(t, pki, pki.doc(nonce, pcr0, time.Now()), pki.leafKey)

	opts := baseOpts(pki, nonce, pcr0)
	opts.ExpectedNonce = nil
	opts.InsecureSkipNonce = true // explicit opt-out
	if _, err := Verify(docB64, opts); err != nil {
		t.Fatalf("InsecureSkipNonce should allow a missing nonce: %v", err)
	}
}

func TestVerify_wrongUserData(t *testing.T) {
	pki := mintPKI(t)
	nonce := []byte("verifier-challenge-32-bytes-xxxx")
	pcr0 := make([]byte, 48)
	docB64 := signDocWith(t, pki, pki.doc(nonce, pcr0, time.Now()), pki.leafKey)

	opts := baseOpts(pki, nonce, pcr0)
	opts.ExpectedUserData = []byte("some/other/protocol")
	if _, err := Verify(docB64, opts); err == nil {
		t.Fatal("wrong user_data accepted")
	}
}

func TestVerify_wrongDigest(t *testing.T) {
	pki := mintPKI(t)
	nonce := []byte("verifier-challenge-32-bytes-xxxx")
	pcr0 := make([]byte, 48)
	d := pki.doc(nonce, pcr0, time.Now())
	d.Digest = "SHA256" // not SHA384
	docB64 := signDocWith(t, pki, d, pki.leafKey)

	if _, err := Verify(docB64, baseOpts(pki, nonce, pcr0)); err == nil {
		t.Fatal("wrong PCR digest accepted")
	}
}

func TestVerify_malformedPCR(t *testing.T) {
	pki := mintPKI(t)
	nonce := []byte("verifier-challenge-32-bytes-xxxx")
	d := pki.doc(nonce, make([]byte, 48), time.Now())
	d.PCRs = map[uint][]byte{0: make([]byte, 32)} // wrong length in the document
	docB64 := signDocWith(t, pki, d, pki.leafKey)

	opts := baseOpts(pki, nonce, make([]byte, 48))
	opts.ExpectedPCRs = nil // no expectation; the document-side length check must still fire
	if _, err := Verify(docB64, opts); err == nil {
		t.Fatal("malformed PCR length accepted")
	}
}

func TestVerify_wrongAlg(t *testing.T) {
	pki := mintPKI(t)
	nonce := []byte("verifier-challenge-32-bytes-xxxx")
	pcr0 := make([]byte, 48)
	d := pki.doc(nonce, pcr0, time.Now())

	payload, _ := cbor.Marshal(d)
	protected, _ := cbor.Marshal(map[int]int{1: -7}) // ES256, not ES384
	sigStructure := []interface{}{"Signature1", protected, []byte{}, payload}
	toSign, _ := cbor.Marshal(sigStructure)
	digest := sha512.Sum384(toSign)
	r, s, _ := ecdsa.Sign(rand.Reader, pki.leafKey, digest[:])
	unprotected, _ := cbor.Marshal(map[int]int{})
	c := coseSign1{Protected: protected, Unprotected: unprotected, Payload: payload, Signature: rawSig(r, s)}
	raw, _ := cbor.Marshal(cbor.Tag{Number: 18, Content: c})
	docB64 := base64.StdEncoding.EncodeToString(raw)

	if _, err := Verify(docB64, baseOpts(pki, nonce, pcr0)); err == nil {
		t.Fatal("wrong protected alg accepted")
	}
}
