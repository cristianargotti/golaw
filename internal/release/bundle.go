package release

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// sigstoreBundle is the part of a Sigstore bundle the verifier reads, in
// the protobuf JSON form of version 0.3 that cosign writes with the new
// bundle format and the attestation action writes: the leaf certificate,
// the log entries with their signed entry timestamps, and the message
// signature or the DSSE envelope. Every other field is ignored, so a newer
// writer that adds one keeps verifying.
type sigstoreBundle struct {
	VerificationMaterial verificationMaterial `json:"verificationMaterial"`
	MessageSignature     *messageSignature    `json:"messageSignature"`
	DSSEEnvelope         *dsseEnvelope        `json:"dsseEnvelope"`
}

type verificationMaterial struct {
	Certificate *rawCertificate `json:"certificate"`
	TlogEntries []tlogEntry     `json:"tlogEntries"`
}

type rawCertificate struct {
	RawBytes string `json:"rawBytes"`
}

type tlogEntry struct {
	LogIndex          protoInt          `json:"logIndex"`
	LogID             logID             `json:"logId"`
	IntegratedTime    protoInt          `json:"integratedTime"`
	InclusionPromise  *inclusionPromise `json:"inclusionPromise"`
	CanonicalizedBody string            `json:"canonicalizedBody"`
}

type logID struct {
	KeyID string `json:"keyId"`
}

type inclusionPromise struct {
	SignedEntryTimestamp string `json:"signedEntryTimestamp"`
}

type messageSignature struct {
	Signature string `json:"signature"`
}

// dsseEnvelope is the signed envelope of an attestation bundle.
type dsseEnvelope struct {
	Payload     string          `json:"payload"`
	PayloadType string          `json:"payloadType"`
	Signatures  []dsseSignature `json:"signatures"`
}

type dsseSignature struct {
	Sig string `json:"sig"`
}

// protoInt is an int64 of the protobuf JSON form, a quoted string of
// digits; a bare number is read as well.
type protoInt int64

// UnmarshalJSON reads the quoted or the bare form.
func (p *protoInt) UnmarshalJSON(data []byte) error {
	n, err := strconv.ParseInt(strings.Trim(string(data), `"`), 10, 64)
	if err != nil {
		return fmt.Errorf("%w: %s is not an integer", errSignature, data)
	}
	*p = protoInt(n)
	return nil
}

// decodeBundle reads one bundle; anything that is not one is an invalid
// signature, named.
func decodeBundle(data []byte) (sigstoreBundle, error) {
	var b sigstoreBundle
	if err := json.Unmarshal(data, &b); err != nil {
		return sigstoreBundle{}, fmt.Errorf("%w: the bundle does not decode: %w", errSignature, err)
	}
	return b, nil
}

// b64 is the standard base64 a log signs a body in.
func b64(data []byte) string { return base64.StdEncoding.EncodeToString(data) }

// decodeBundles reads every bundle of an attestation file: one JSON
// document, as a bundle saved on its own, or one per line, as the file gh
// attestation download writes.
func decodeBundles(data []byte) ([]sigstoreBundle, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var out []sigstoreBundle
	for {
		var b sigstoreBundle
		err := decoder.Decode(&b)
		switch {
		case errors.Is(err, io.EOF) && len(out) > 0:
			return out, nil
		case errors.Is(err, io.EOF):
			return nil, fmt.Errorf("%w: the file holds no bundle", errSignature)
		case err != nil:
			return nil, fmt.Errorf("%w: the bundle does not decode: %w", errSignature, err)
		}
		out = append(out, b)
	}
}

// decode64 reads the standard base64 the protobuf JSON form writes for
// bytes.
func decode64(field, text string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return nil, fmt.Errorf("%w: %s is not base64: %w", errSignature, field, err)
	}
	return data, nil
}
