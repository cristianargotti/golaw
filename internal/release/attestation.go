package release

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// payloadType is the type of an in-toto statement in a DSSE envelope, and
// kindDSSE the kind of its log entry.
const (
	payloadType = "application/vnd.in-toto+json"
	kindDSSE    = "dsse"
)

// recordedVerifier is one signature a dsse entry records, with the
// certificate that verifies it.
type recordedVerifier struct {
	Signature string `json:"signature"`
	Verifier  string `json:"verifier"`
}

// statement is the part of an in-toto statement the verifier reads: the
// subjects, each a name and its digests. The predicate is never read.
type statement struct {
	Subject []subject `json:"subject"`
}

type subject struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

// attestationStep names the asset and the cause; an attestation signed by
// another signer is an identity mismatch, any other failure a mismatch.
func attestationStep(in Inputs, v Verdict) (Verdict, bool) {
	if len(in.Attestation) == 0 {
		return red(v, CodeAttestationMissing, "no attestation of "+in.Asset+" in the release directory: download it with gh attestation download"), false
	}
	err := attested(in)
	switch {
	case err == nil:
		return v, true
	case errors.Is(err, errIdentity):
		return red(v, CodeIdentityMismatch, "the attestation of "+in.Asset+": "+err.Error()), false
	}
	return red(v, CodeAttestationMismatch, "the attestation of "+in.Asset+": "+err.Error()), false
}

// attested accepts the first bundle of the attestation file that verifies
// for the asset; when none does, the first one's failure is the answer.
func attested(in Inputs) error {
	bundles, err := decodeBundles(in.Attestation)
	if err != nil {
		return err
	}
	var first error
	for _, b := range bundles {
		err := oneAttestation(b, in)
		if err == nil {
			return nil
		}
		if first == nil {
			first = err
		}
	}
	return first
}

// oneAttestation proves one bundle: its log entry and certificate as for
// checksums.txt, its envelope signed by that certificate and recorded by
// its log entry, and its statement naming the asset at the running
// binary's digest.
func oneAttestation(b sigstoreBundle, in Inputs) error {
	env := b.DSSEEnvelope
	if env == nil || env.PayloadType != payloadType || len(env.Signatures) == 0 {
		return fmt.Errorf("%w: the bundle carries no signed in-toto envelope", errSignature)
	}
	cert, err := signer(b, in)
	if err != nil {
		return err
	}
	payload, err := decode64("the payload", env.Payload)
	if err != nil {
		return err
	}
	sig, err := decode64("the envelope signature", env.Signatures[0].Sig)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(fmt.Appendf(nil, "DSSEv1 %d %s %d %s", len(payloadType), payloadType, len(payload), payload))
	if err = verifyECDSA(cert.PublicKey, sum[:], sig); err != nil {
		return fmt.Errorf("%w over the envelope", err)
	}
	r, err := recorded(b.VerificationMaterial.TlogEntries)
	if err != nil {
		return err
	}
	if err := dsseRecords(r, cert, sig, payload); err != nil {
		return err
	}
	return names(payload, in.Asset, in.BinaryDigest)
}

// dsseRecords requires the dsse entry of the attestation to record this
// payload and this envelope signature by this certificate.
func dsseRecords(r recordedEntry, cert *x509.Certificate, sig, payload []byte) error {
	sum := sha256.Sum256(payload)
	signed := slices.ContainsFunc(r.Spec.Signatures, func(v recordedVerifier) bool {
		return sameBase64(v.Signature, sig) && sameCertificate(v.Verifier, cert)
	})
	if r.Kind != kindDSSE || !signed || !strings.EqualFold(r.Spec.PayloadHash.Value, hex.EncodeToString(sum[:])) {
		return fmt.Errorf("%w: the log entry does not record this envelope", errAttestation)
	}
	return nil
}

// names requires a subject of the statement to be the asset at digest.
func names(payload []byte, asset, digest string) error {
	var s statement
	if err := json.Unmarshal(payload, &s); err != nil {
		return fmt.Errorf("%w: the statement does not decode: %w", errAttestation, err)
	}
	for _, sub := range s.Subject {
		if sub.Name == asset && strings.EqualFold(sub.Digest["sha256"], digest) {
			return nil
		}
	}
	return fmt.Errorf("%w: no subject of the statement is %s at sha256 %s", errAttestation, asset, digest)
}
