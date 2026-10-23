package release

import "errors"

var (
	// ErrInput marks a release input the caller could not read: a file of
	// the release directory or the running executable.
	ErrInput = errors.New("release input")
	// errSignature marks a bundle that does not prove what it claims: a
	// part that does not decode, a certificate outside the trust root, a
	// signature or a log entry that does not verify.
	errSignature = errors.New("invalid signature")
	// errIdentity marks a certificate that verifies and names another
	// signer than the pinned identity, a code of its own (D11).
	errIdentity = errors.New("identity mismatch")
	// errAttestation marks an attestation that verifies and names another
	// asset or another digest than the running binary.
	errAttestation = errors.New("attestation mismatch")
)
