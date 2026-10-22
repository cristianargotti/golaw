package release

import (
	"crypto"
	"crypto/x509"
	"time"
)

// The verdict codes of the release line of doctor (D11), one per outcome.
const (
	CodeVerified            = "release-verified"
	CodeChecksumMismatch    = "checksum-mismatch"
	CodeSignatureInvalid    = "signature-invalid"
	CodeIdentityMismatch    = "identity-mismatch"
	CodeAttestationMissing  = "attestation-missing"
	CodeAttestationMismatch = "attestation-mismatch"
	CodeTrustRootInvalid    = "trust-root-invalid"
	CodeInputMissing        = "release-input-missing"
	CodeSourceBuild         = "source-build"
	CodeUnverified          = "release-unverified"
)

// The states of a verdict: verified, refused, or not decided.
const (
	StateOK      = "ok"
	StateRed     = "red"
	StateUnknown = "unknown"
)

// The files of a release and the identity the lane signs under.
const (
	ChecksumsFile = "checksums.txt"
	BundleSuffix  = ".sigstore.json"
	DefaultIssuer = "https://token.actions.githubusercontent.com"
	Lane          = "https://github.com/cristianargotti/golaw/.github/workflows/release.yml"
)

// Identity is the signing identity a release must carry (section 4.2): the
// lane file and the OIDC issuer, bound once by cmd and replaced by a test
// identity in every test.
type Identity struct {
	Lane   string // the lane file URL
	Issuer string // the OIDC issuer
}

// Subject is the subject alternative name of a release tag: the lane at
// refs/tags/v<version>.
func (i Identity) Subject(version string) string {
	return i.Lane + "@refs/tags/v" + version
}

// TrustRoot is what a certificate and a log entry are verified against:
// the certificate authority roots and the public key of each transparency
// log by its id, the hex SHA-256 of the DER public key.
type TrustRoot struct {
	Roots *x509.CertPool
	Rekor map[string]crypto.PublicKey
}

// Inputs is what Verify judges; every file is bytes the caller read.
type Inputs struct {
	BinaryDigest string    // the hex SHA-256 of the executable
	Asset        string    // the asset name of the running binary
	Version      string    // the release version X.Y.Z; empty for a development build
	Checksums    []byte    // checksums.txt
	Bundle       []byte    // checksums.txt.sigstore.json
	Attestation  []byte    // the attestation bundle of the asset
	Identity     Identity  // the identity the signature must carry
	Trust        TrustRoot // the roots and the log keys
	Now          time.Time // the clock, for the certificate validity at signing time
}

// Verdict is the one line doctor prints for the release.
type Verdict struct {
	State  string // StateOK, StateRed or StateUnknown
	Code   string // one of the codes above
	Detail string // what was judged and, when red, what differs
	Digest string // the digest judged
	Signer string // the subject alternative name that verified, empty otherwise
}
