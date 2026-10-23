package releasetest

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/cristianargotti/golaw/internal/release"
)

// The forms the lane's tools write: the bundle media type of cosign with
// the new bundle format and of the attestation action, and the in-toto
// statement of a build provenance attestation.
const (
	bundleMediaType  = "application/vnd.dev.sigstore.bundle.v0.3+json"
	payloadType      = "application/vnd.in-toto+json"
	statementType    = "https://in-toto.io/Statement/v1"
	provenanceType   = "https://slsa.dev/provenance/v1"
	checksumsIndex   = 1000
	attestationIndex = 1001
	entryVersion     = "0.0.1"
	kindRekord       = "hashedrekord"
	kindDSSE         = "dsse"
	keySignature     = "signature"
)

// writeBundle writes the bundle of checksums.txt as cosign writes it: a
// leaf for id at the release tag, the signature over the SHA-256 of
// signed (checksums.txt itself unless a tamper signs other bytes), the
// hashedrekord entry of the log, in the v0.3 form.
func (m Material) writeBundle(tb testing.TB, id release.Identity, signed []byte) {
	tb.Helper()
	m.writeRecordedBundle(tb, id, signed, signed)
}

// writeRecordedBundle is writeBundle with a log entry that records the
// signature of recorded by the same leaf: signed itself, unless a tamper
// logs another signature, which the log still signs.
func (m Material) writeRecordedBundle(tb testing.TB, id release.Identity, signed, recorded []byte) {
	tb.Helper()
	leaf, key := m.keys.leaf(tb, id.Subject(m.Version), id.Issuer)
	sum := sha256.Sum256(signed)
	sig := sign(tb, key, signed)
	logged := sig
	if !bytes.Equal(recorded, signed) {
		logged = sign(tb, key, recorded)
	}
	body := entryBody(tb, kindRekord, map[string]any{
		"data":       map[string]any{"hash": hashOf(recorded)},
		keySignature: map[string]any{"content": b64(logged), "publicKey": map[string]string{"content": b64(pemCertificate(leaf))}},
	})
	write(tb, m.Bundle, marshal(tb, map[string]any{
		"mediaType":            bundleMediaType,
		"verificationMaterial": m.material(tb, leaf.Raw, m.keys.entry(tb, kindRekord, body, checksumsIndex)),
		"messageSignature": map[string]any{
			"messageDigest": map[string]string{"algorithm": "SHA2_256", "digest": b64(sum[:])},
			keySignature:    b64(sig),
		},
	}))
}

// writeAttestation writes the provenance attestation of the asset as the
// attestation action does: an in-toto statement naming the asset at
// digest, in a DSSE envelope signed by a leaf for id, with its dsse entry.
func (m Material) writeAttestation(tb testing.TB, id release.Identity, digest string) {
	tb.Helper()
	m.writeRecordedAttestation(tb, id, digest, digest)
}

// writeRecordedAttestation is writeAttestation with a dsse entry that
// records the envelope of the statement for recorded by the same leaf:
// the envelope itself, unless a tamper logs another one, which the log
// still signs.
func (m Material) writeRecordedAttestation(tb testing.TB, id release.Identity, digest, recorded string) {
	tb.Helper()
	leaf, key := m.keys.leaf(tb, id.Subject(m.Version), id.Issuer)
	envelope, statement, sig := m.envelope(tb, key, digest)
	logged, loggedStatement, loggedSig := envelope, statement, sig
	if recorded != digest {
		logged, loggedStatement, loggedSig = m.envelope(tb, key, recorded)
	}
	body := entryBody(tb, kindDSSE, map[string]any{
		"envelopeHash": hashOf(marshal(tb, logged)),
		"payloadHash":  hashOf(loggedStatement),
		"signatures":   []any{map[string]string{keySignature: b64(loggedSig), "verifier": b64(pemCertificate(leaf))}},
	})
	write(tb, m.Attestation, marshal(tb, map[string]any{
		"mediaType":            bundleMediaType,
		"verificationMaterial": m.material(tb, leaf.Raw, m.keys.entry(tb, kindDSSE, body, attestationIndex)),
		"dsseEnvelope":         envelope,
	}))
}

// envelope is the DSSE envelope of the provenance statement that names
// the asset at digest, signed by key, with the statement and the
// signature.
func (m Material) envelope(tb testing.TB, key *ecdsa.PrivateKey, digest string) (map[string]any, []byte, []byte) {
	tb.Helper()
	statement := marshal(tb, map[string]any{
		"_type":         statementType,
		"subject":       []any{map[string]any{"name": m.Asset, "digest": map[string]string{"sha256": digest}}},
		"predicateType": provenanceType,
		"predicate":     map[string]any{"buildDefinition": map[string]string{"buildType": "https://example.test/build"}},
	})
	sig := sign(tb, key, pae(statement))
	return map[string]any{
		"payload": b64(statement), "payloadType": payloadType,
		"signatures": []any{map[string]string{"sig": b64(sig), "keyid": ""}},
	}, statement, sig
}

// material is the verification material of a bundle: the leaf in the
// v0.3 form and its one log entry.
func (m Material) material(tb testing.TB, leaf []byte, entry map[string]any) map[string]any {
	tb.Helper()
	return map[string]any{
		"certificate": map[string]string{"rawBytes": b64(leaf)},
		"tlogEntries": []any{entry},
	}
}

// entryBody is the body of a log entry of kind, in the canonical JSON the
// log keeps.
func entryBody(tb testing.TB, kind string, spec map[string]any) []byte {
	tb.Helper()
	return marshal(tb, map[string]any{"apiVersion": entryVersion, "kind": kind, "spec": spec})
}

// hashOf is the hash object of a log body: the algorithm and the hex
// SHA-256 of data.
func hashOf(data []byte) map[string]string {
	return map[string]string{"algorithm": "sha256", "value": hexSum(data)}
}

// pae is the pre-authentication encoding DSSE signs.
func pae(payload []byte) []byte {
	return fmt.Appendf(nil, "DSSEv1 %d %s %d %s", len(payloadType), payloadType, len(payload), payload)
}

// marshal is the JSON of a value the helpers build; a map's keys come out
// sorted, the canonical form a log body takes.
func marshal(tb testing.TB, value any) []byte {
	tb.Helper()
	data, err := json.Marshal(value)
	must(tb, err)
	return data
}
