package main

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
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/url"
	"strconv"
	"time"

	"github.com/cristianargotti/golaw/internal/release"
)

// The formats of the Sigstore bundle the lane writes (goreleaser v2.12.7
// with cosign v2.6.5, bundle v0.3), of an in-toto statement and of the two
// log entry kinds, with the keys several of them share.
const (
	m5BundleMedia    = "application/vnd.dev.sigstore.bundle.v0.3+json"
	m5StatementMedia = "application/vnd.in-toto+json"
	m5EntryVersion   = "0.0.1"
	m5Certificate    = "CERTIFICATE"
	m5SHA256         = "sha256"
	m5KeySignature   = "signature"
	m5KeyAlgorithm   = "algorithm"
	m5KeyContent     = "content"
	m5KeyValue       = "value"
	m5KeyDigest      = "digest"
	m5KeyKind        = "kind"
)

// m5IssuerOID is the Fulcio extension of the OIDC issuer as a DER
// UTF8String; m5LegacyIssuerOID carries it raw, as older certificates do.
func m5IssuerOID() asn1.ObjectIdentifier { return asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 8} }

func m5LegacyIssuerOID() asn1.ObjectIdentifier {
	return asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 1}
}

// m5Authority is the test certificate authority and transparency log of
// generated material, every signature made at one integrated time.
type m5Authority struct {
	root     *x509.Certificate
	rootKey  *ecdsa.PrivateKey
	rekorKey *ecdsa.PrivateKey
	logID    []byte
	at       time.Time
}

func m5NewAuthority(at time.Time) (*m5Authority, error) {
	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate the root key: %w", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "golaw acceptance root"},
		NotBefore: at.Add(-time.Hour), NotAfter: at.Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &rootKey.PublicKey, rootKey)
	if err != nil {
		return nil, fmt.Errorf("create the root: %w", err)
	}
	root, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("parse the root: %w", err)
	}
	rekorKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate the log key: %w", err)
	}
	public, err := x509.MarshalPKIXPublicKey(&rekorKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("encode the log key: %w", err)
	}
	id := sha256.Sum256(public)
	return &m5Authority{root: root, rootKey: rootKey, rekorKey: rekorKey, logID: id[:], at: at}, nil
}

// leaf issues the short-lived code signing certificate of identity at the
// tag of version, with the issuer in both extension forms.
func (a *m5Authority) leaf(identity release.Identity, version string) ([]byte, *ecdsa.PrivateKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate the leaf key: %w", err)
	}
	subject, err := url.Parse(identity.Subject(version))
	if err != nil {
		return nil, nil, fmt.Errorf("parse the subject: %w", err)
	}
	issuer, err := asn1.MarshalWithParams(identity.Issuer, "utf8")
	if err != nil {
		return nil, nil, fmt.Errorf("encode the issuer: %w", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(a.at.UnixNano()), NotBefore: a.at.Add(-time.Minute), NotAfter: a.at.Add(10 * time.Minute),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		URIs: []*url.URL{subject},
		ExtraExtensions: []pkix.Extension{
			{Id: m5IssuerOID(), Value: issuer}, {Id: m5LegacyIssuerOID(), Value: []byte(identity.Issuer)},
		},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, a.root, &key.PublicKey, a.rootKey)
	if err != nil {
		return nil, nil, fmt.Errorf("issue the leaf: %w", err)
	}
	return der, key, nil
}

// entry is a log entry: the canonical body and the signed entry timestamp
// the log key makes over the canonical JSON of body, integratedTime, logID
// and logIndex, keys sorted as json.Marshal writes this struct.
func (a *m5Authority) entry(kind string, body []byte, index int64) (map[string]any, error) {
	encoded := base64.StdEncoding.EncodeToString(body)
	payload, err := json.Marshal(struct {
		Body           string `json:"body"`
		IntegratedTime int64  `json:"integratedTime"`
		LogID          string `json:"logID"`
		LogIndex       int64  `json:"logIndex"`
	}{encoded, a.at.Unix(), hex.EncodeToString(a.logID), index})
	if err != nil {
		return nil, fmt.Errorf("encode the entry payload: %w", err)
	}
	sum := sha256.Sum256(payload)
	timestamp, err := ecdsa.SignASN1(rand.Reader, a.rekorKey, sum[:])
	if err != nil {
		return nil, fmt.Errorf("sign the entry: %w", err)
	}
	return map[string]any{
		"logIndex": strconv.FormatInt(index, 10), "logId": map[string]any{"keyId": base64.StdEncoding.EncodeToString(a.logID)},
		"kindVersion": map[string]any{m5KeyKind: kind, "version": m5EntryVersion}, "integratedTime": strconv.FormatInt(a.at.Unix(), 10),
		"inclusionPromise":  map[string]any{"signedEntryTimestamp": base64.StdEncoding.EncodeToString(timestamp)},
		"canonicalizedBody": encoded,
	}, nil
}

// material is the verification material of a bundle: the leaf and its entry.
func (a *m5Authority) material(leaf []byte, kind string, body []byte, index int64) (map[string]any, error) {
	entry, err := a.entry(kind, body, index)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"certificate": map[string]any{"rawBytes": base64.StdEncoding.EncodeToString(leaf)},
		"tlogEntries": []any{entry},
	}, nil
}

// m5Hash is the hash object of an entry body.
func m5Hash(sum []byte) map[string]any {
	return map[string]any{m5KeyAlgorithm: m5SHA256, m5KeyValue: hex.EncodeToString(sum)}
}

// messageBundle signs checksums.txt as cosign sign-blob --new-bundle-format
// does: a hashedrekord entry and the message signature over its SHA-256.
func (a *m5Authority) messageBundle(checksums, leaf []byte, key *ecdsa.PrivateKey) ([]byte, error) {
	sum := sha256.Sum256(checksums)
	signature, err := ecdsa.SignASN1(rand.Reader, key, sum[:])
	if err != nil {
		return nil, fmt.Errorf("sign checksums.txt: %w", err)
	}
	certificate := base64.StdEncoding.EncodeToString(m5PEM(m5Certificate, leaf))
	body, err := json.Marshal(map[string]any{"apiVersion": m5EntryVersion, m5KeyKind: "hashedrekord", "spec": map[string]any{
		"data": map[string]any{"hash": m5Hash(sum[:])},
		m5KeySignature: map[string]any{
			m5KeyContent: base64.StdEncoding.EncodeToString(signature),
			"publicKey":  map[string]any{m5KeyContent: certificate},
		},
	}})
	if err != nil {
		return nil, fmt.Errorf("encode the entry body: %w", err)
	}
	material, err := a.material(leaf, "hashedrekord", body, 1)
	if err != nil {
		return nil, err
	}
	return m5JSON(map[string]any{
		"mediaType": m5BundleMedia, "verificationMaterial": material,
		"messageSignature": map[string]any{
			"messageDigest": map[string]any{m5KeyAlgorithm: "SHA2_256", m5KeyDigest: base64.StdEncoding.EncodeToString(sum[:])},
			m5KeySignature:  base64.StdEncoding.EncodeToString(signature),
		},
	})
}

// attestationBundle is the provenance attestation of the asset: an in-toto
// statement naming it with its digest in a DSSE envelope signed over the
// pre-authentication encoding, and its dsse entry.
func (a *m5Authority) attestationBundle(asset, digest string, leaf []byte, key *ecdsa.PrivateKey) ([]byte, error) {
	statement, err := json.Marshal(map[string]any{
		"_type": "https://in-toto.io/Statement/v1", "predicateType": "https://slsa.dev/provenance/v1",
		"subject":   []any{map[string]any{"name": asset, m5KeyDigest: map[string]any{m5SHA256: digest}}},
		"predicate": map[string]any{"buildDefinition": map[string]any{"buildType": "https://example.test/acceptance"}},
	})
	if err != nil {
		return nil, fmt.Errorf("encode the statement: %w", err)
	}
	pae := fmt.Sprintf("DSSEv1 %d %s %d %s", len(m5StatementMedia), m5StatementMedia, len(statement), statement)
	sum := sha256.Sum256([]byte(pae))
	signature, err := ecdsa.SignASN1(rand.Reader, key, sum[:])
	if err != nil {
		return nil, fmt.Errorf("sign the statement: %w", err)
	}
	envelope := map[string]any{
		"payload": base64.StdEncoding.EncodeToString(statement), "payloadType": m5StatementMedia,
		"signatures": []any{map[string]any{"sig": base64.StdEncoding.EncodeToString(signature)}},
	}
	body, err := m5DSSEBody(envelope, statement, signature, leaf)
	if err != nil {
		return nil, err
	}
	material, err := a.material(leaf, "dsse", body, 2)
	if err != nil {
		return nil, err
	}
	return m5JSON(map[string]any{"mediaType": m5BundleMedia, "verificationMaterial": material, "dsseEnvelope": envelope})
}

// m5DSSEBody is the canonical body of a dsse log entry.
func m5DSSEBody(envelope map[string]any, statement, signature, leaf []byte) ([]byte, error) {
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("encode the envelope: %w", err)
	}
	envelopeSum, statementSum := sha256.Sum256(encoded), sha256.Sum256(statement)
	body, err := json.Marshal(map[string]any{"apiVersion": m5EntryVersion, m5KeyKind: "dsse", "spec": map[string]any{
		"envelopeHash": m5Hash(envelopeSum[:]), "payloadHash": m5Hash(statementSum[:]),
		"signatures": []any{map[string]any{
			m5KeySignature: base64.StdEncoding.EncodeToString(signature),
			"verifier":     base64.StdEncoding.EncodeToString(m5PEM(m5Certificate, leaf)),
		}},
	}})
	if err != nil {
		return nil, fmt.Errorf("encode the entry body: %w", err)
	}
	return body, nil
}

// trustFiles are roots.crt, the root in PEM, and rekor.pub, the log key in
// PEM, the two files of a trust root directory.
func (a *m5Authority) trustFiles() ([]byte, []byte, error) {
	public, err := x509.MarshalPKIXPublicKey(&a.rekorKey.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("encode the log key: %w", err)
	}
	return m5PEM(m5Certificate, a.root.Raw), m5PEM("PUBLIC KEY", public), nil
}

func m5PEM(kind string, der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: der})
}

// m5JSON encodes a document of the material.
func m5JSON(value any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode the material: %w", err)
	}
	return encoded, nil
}
