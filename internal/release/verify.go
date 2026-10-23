package release

import (
	"errors"
	"fmt"
)

// step is one judgment of Verify: the verdict and false when the release
// fails it, true to go on to the next.
type step func(in Inputs, v Verdict) (Verdict, bool)

// Verify answers the verdict of a release (D11), in the order a person
// would check by hand: a development build is a source build and is never
// verified; then checksums.txt and its bundle must be there, the asset
// line must name the running binary's digest, the trust root must hold an
// authority and a log key, and the bundle must sign checksums.txt under
// the pinned identity. What passes them all is not verified yet: the log
// and attestation steps follow, and until they do the verdict says so.
func Verify(in Inputs) Verdict {
	v := Verdict{Digest: in.BinaryDigest}
	for _, judge := range []step{sourceStep, inputStep, checksumStep, trustStep, signatureStep} {
		next, ok := judge(in, v)
		if !ok {
			return next
		}
	}
	return unknown(v, CodeUnverified, "the log entry and the attestation are not verified yet")
}

func sourceStep(in Inputs, v Verdict) (Verdict, bool) {
	if in.Version == "" {
		return unknown(v, CodeSourceBuild, "a source build reports metadata and is never verified"), false
	}
	return v, true
}

func inputStep(in Inputs, v Verdict) (Verdict, bool) {
	switch {
	case len(in.Checksums) == 0:
		return red(v, CodeInputMissing, ChecksumsFile+" is missing from the release directory"), false
	case len(in.Bundle) == 0:
		return red(v, CodeInputMissing, ChecksumsFile+BundleSuffix+" is missing from the release directory"), false
	}
	return v, true
}

func checksumStep(in Inputs, v Verdict) (Verdict, bool) {
	expected, found := ChecksumOf(in.Checksums, in.Asset)
	switch {
	case !found:
		return red(v, CodeChecksumMismatch, ChecksumsFile+" has no line for "+in.Asset), false
	case expected != in.BinaryDigest:
		return red(v, CodeChecksumMismatch, fmt.Sprintf("%s hashes to %s and %s says %s", in.Asset, in.BinaryDigest, ChecksumsFile, expected)), false
	}
	return v, true
}

func trustStep(in Inputs, v Verdict) (Verdict, bool) {
	if in.Trust.Roots == nil || len(in.Trust.Rekor) == 0 {
		return red(v, CodeTrustRootInvalid, "the trust root holds no certificate authority or no log key"), false
	}
	return v, true
}

// signatureStep names the bundle and the cause; a certificate for another
// signer is an identity mismatch, anything else an invalid signature.
func signatureStep(in Inputs, v Verdict) (Verdict, bool) {
	err := checksumsSigned(in)
	switch {
	case err == nil:
		return v, true
	case errors.Is(err, errIdentity):
		return red(v, CodeIdentityMismatch, ChecksumsFile+BundleSuffix+": "+err.Error()), false
	}
	return red(v, CodeSignatureInvalid, ChecksumsFile+BundleSuffix+": "+err.Error()), false
}

// red is a refused verdict with its code and what differs.
func red(v Verdict, code, detail string) Verdict {
	v.State, v.Code, v.Detail = StateRed, code, detail
	return v
}

// unknown is a verdict that decides nothing, with why.
func unknown(v Verdict, code, detail string) Verdict {
	v.State, v.Code, v.Detail = StateUnknown, code, detail
	return v
}
