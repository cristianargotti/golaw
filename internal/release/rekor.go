package release

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strings"
	"time"
)

// kindRekord is the kind of the log entry of a signed blob.
const kindRekord = "hashedrekord"

// recordedEntry is the part of a log entry body the verifier compares
// with the bundle, so an entry the log signed for another signature never
// stands for this one: a hashedrekord records the digest signed, the
// signature and the certificate.
type recordedEntry struct {
	Kind string       `json:"kind"`
	Spec recordedSpec `json:"spec"`
}

type recordedSpec struct {
	Data        recordedData       `json:"data"`
	Signature   recordedSignature  `json:"signature"`
	PayloadHash recordedHash       `json:"payloadHash"`
	Signatures  []recordedVerifier `json:"signatures"`
}

type recordedData struct {
	Hash recordedHash `json:"hash"`
}

type recordedHash struct {
	Value string `json:"value"`
}

type recordedSignature struct {
	Content   string      `json:"content"`
	PublicKey recordedKey `json:"publicKey"`
}

type recordedKey struct {
	Content string `json:"content"`
}

// logged verifies the first log entry of a bundle and answers the time
// the log integrated the signature, the time every certificate is judged
// at: its signed entry timestamp must verify under the key the trust root
// holds for the entry's log, over the canonical JSON of the body, the
// time, the log id and the index (D11). The inclusion proof is not read:
// the timestamp binds the entry to the log, a stated limit of the docs;
// what the entry records is compared with the bundle by the step that
// verifies the signature (recorded).
func logged(entries []tlogEntry, trust TrustRoot) (time.Time, error) {
	if len(entries) == 0 {
		return time.Time{}, fmt.Errorf("%w: the bundle carries no transparency log entry", errSignature)
	}
	e := entries[0]
	if e.InclusionPromise == nil {
		return time.Time{}, fmt.Errorf("%w: the log entry carries no signed entry timestamp; this version verifies the entries of a log that promises inclusion", errSignature)
	}
	id, err := decode64("the log id", e.LogID.KeyID)
	if err != nil {
		return time.Time{}, err
	}
	key, ok := trust.Rekor[hex.EncodeToString(id)]
	if !ok {
		return time.Time{}, fmt.Errorf("%w: the log entry names the log %s, which the trust root does not hold", errSignature, hex.EncodeToString(id))
	}
	body, err := decode64("the log entry body", e.CanonicalizedBody)
	if err != nil {
		return time.Time{}, err
	}
	set, err := decode64("the signed entry timestamp", e.InclusionPromise.SignedEntryTimestamp)
	if err != nil {
		return time.Time{}, err
	}
	sum := sha256.Sum256(entryPayload(e, body, id))
	if err := verifyECDSA(key, sum[:], set); err != nil {
		return time.Time{}, fmt.Errorf("%w for the signed entry timestamp of the log", err)
	}
	return time.Unix(int64(e.IntegratedTime), 0).UTC(), nil
}

// entryPayload is the canonical JSON the log signed for an entry: the keys
// in order, the body in standard base64, the log id in hex.
func entryPayload(e tlogEntry, body, id []byte) []byte {
	return fmt.Appendf(nil, `{"body":%q,"integratedTime":%d,"logID":%q,"logIndex":%d}`,
		b64(body), int64(e.IntegratedTime), hex.EncodeToString(id), int64(e.LogIndex))
}

// recorded decodes the body of the first log entry of a bundle, the entry
// logged verified.
func recorded(entries []tlogEntry) (recordedEntry, error) {
	var r recordedEntry
	if len(entries) == 0 {
		return r, fmt.Errorf("%w: the bundle carries no transparency log entry", errSignature)
	}
	body, err := decode64("the log entry body", entries[0].CanonicalizedBody)
	if err != nil {
		return r, err
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return r, fmt.Errorf("%w: the log entry body does not decode: %w", errSignature, err)
	}
	return r, nil
}

// rekordRecords requires the hashedrekord entry of the bundle to record
// this signature, by this certificate, over this digest.
func rekordRecords(r recordedEntry, cert *x509.Certificate, sig, digest []byte) error {
	if r.Kind != kindRekord || !sameBase64(r.Spec.Signature.Content, sig) ||
		!sameCertificate(r.Spec.Signature.PublicKey.Content, cert) ||
		!strings.EqualFold(r.Spec.Data.Hash.Value, hex.EncodeToString(digest)) {
		return fmt.Errorf("%w: the log entry does not record this signature of %s", errSignature, ChecksumsFile)
	}
	return nil
}

// sameBase64 reports whether text is the standard base64 of want.
func sameBase64(text string, want []byte) bool {
	data, err := base64.StdEncoding.DecodeString(text)
	return err == nil && bytes.Equal(data, want)
}

// sameCertificate reports whether text is the base64 of the PEM form of
// cert, the form a log entry records a certificate in.
func sameCertificate(text string, cert *x509.Certificate) bool {
	data, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return false
	}
	block, _ := pem.Decode(data)
	return block != nil && bytes.Equal(block.Bytes, cert.Raw)
}
