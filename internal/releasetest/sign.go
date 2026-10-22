package releasetest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/url"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// The clock of every generated release: the log integrates each entry at
// integratedUnix (2026-10-01T12:00:00Z), inside the ten minutes the leaf
// is valid, as the public certificate authority issues a leaf to a run.
const (
	integratedUnix = 1790856000
	leafSkew       = 5 * time.Minute
	caSpan         = 10 * 365 * 24 * time.Hour
	serialBits     = 64
)

// signer holds the keys a generated release is signed with: the
// certificate authority, its certificate, and the key of the log.
type signer struct {
	ca    *x509.Certificate
	caKey *ecdsa.PrivateKey
	log   *ecdsa.PrivateKey
}

// newSigner makes a certificate authority valid for ten years around the
// signing time and the key of a transparency log.
func newSigner(tb testing.TB) *signer {
	tb.Helper()
	caKey := newKey(tb)
	template := &x509.Certificate{
		SerialNumber:          serial(tb),
		Subject:               pkix.Name{Organization: []string{"example.test"}, CommonName: "release test root"},
		NotBefore:             integrated().Add(-caSpan),
		NotAfter:              integrated().Add(caSpan),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &caKey.PublicKey, caKey)
	must(tb, err)
	ca, err := x509.ParseCertificate(der)
	must(tb, err)
	return &signer{ca: ca, caKey: caKey, log: newKey(tb)}
}

// leaf issues a certificate for subject as the public certificate
// authority issues one to a lane run: no subject name, the URI, the two
// issuer extensions, the code signing usage, ten minutes of validity.
func (s *signer) leaf(tb testing.TB, subject, issuer string) (*x509.Certificate, *ecdsa.PrivateKey) {
	tb.Helper()
	key := newKey(tb)
	uri, err := url.Parse(subject)
	must(tb, err)
	issuerV2, err := asn1.MarshalWithParams(issuer, "utf8")
	must(tb, err)
	template := &x509.Certificate{
		SerialNumber: serial(tb),
		NotBefore:    integrated().Add(-leafSkew),
		NotAfter:     integrated().Add(leafSkew),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		URIs:         []*url.URL{uri},
		ExtraExtensions: []pkix.Extension{
			{Id: asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 1}, Value: []byte(issuer)},
			{Id: asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 8}, Value: issuerV2},
		},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, s.ca, &key.PublicKey, s.caKey)
	must(tb, err)
	cert, err := x509.ParseCertificate(der)
	must(tb, err)
	return cert, key
}

// entry is the transparency log entry of a body at index: integrated at
// the fixed time, with the signed entry timestamp of the log over the
// canonical JSON of the body, the time, the log id and the index.
func (s *signer) entry(tb testing.TB, kind string, body []byte, index int64) map[string]any {
	tb.Helper()
	der, err := x509.MarshalPKIXPublicKey(&s.log.PublicKey)
	must(tb, err)
	id := sha256.Sum256(der)
	payload := fmt.Appendf(nil, `{"body":%q,"integratedTime":%d,"logID":%q,"logIndex":%d}`,
		b64(body), integratedUnix, hex.EncodeToString(id[:]), index)
	return map[string]any{
		"logIndex":          strconv.FormatInt(index, 10),
		"logId":             map[string]string{"keyId": b64(id[:])},
		"kindVersion":       map[string]string{"kind": kind, "version": entryVersion},
		"integratedTime":    strconv.FormatInt(integratedUnix, 10),
		"inclusionPromise":  map[string]string{"signedEntryTimestamp": b64(sign(tb, s.log, payload))},
		"canonicalizedBody": b64(body),
	}
}

// writeTrust writes the trust root of the material: the certificate of
// the authority and the public key of the log, both PEM.
func (s *signer) writeTrust(tb testing.TB, dir string) {
	tb.Helper()
	key, err := x509.MarshalPKIXPublicKey(&s.log.PublicKey)
	must(tb, err)
	write(tb, filepath.Join(dir, rootsFile), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: s.ca.Raw}))
	write(tb, filepath.Join(dir, rekorFile), pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: key}))
}

// sign is the ASN.1 ECDSA signature of key over the SHA-256 of message.
func sign(tb testing.TB, key *ecdsa.PrivateKey, message []byte) []byte {
	tb.Helper()
	sum := sha256.Sum256(message)
	sig, err := ecdsa.SignASN1(rand.Reader, key, sum[:])
	must(tb, err)
	return sig
}

func newKey(tb testing.TB) *ecdsa.PrivateKey {
	tb.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	must(tb, err)
	return key
}

func serial(tb testing.TB) *big.Int {
	tb.Helper()
	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), serialBits))
	must(tb, err)
	return n
}

// integrated is the time the log integrates every generated entry.
func integrated() time.Time { return time.Unix(integratedUnix, 0).UTC() }

// b64 is the standard base64 the protobuf JSON form writes for bytes.
func b64(data []byte) string { return base64.StdEncoding.EncodeToString(data) }

// pemCertificate is the PEM form of a certificate, as a log body names it.
func pemCertificate(cert *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
}
