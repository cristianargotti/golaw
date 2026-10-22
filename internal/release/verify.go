package release

import "fmt"

// Verify answers the verdict of a release (D11), in the order a person
// would check by hand: a development build is a source build and is never
// verified; a release without checksums.txt or its bundle is missing an
// input; an asset absent from checksums.txt or hashing to another digest
// is a checksum mismatch. What passes these steps is not verified yet: the
// certificate, signature, log and attestation steps replace the last
// answer, and until they do the verdict says so instead of claiming.
func Verify(in Inputs) Verdict {
	v := Verdict{Digest: in.BinaryDigest}
	if in.Version == "" {
		return unknown(v, CodeSourceBuild, "a source build reports metadata and is never verified")
	}
	if len(in.Checksums) == 0 {
		return red(v, CodeInputMissing, ChecksumsFile+" is missing from the release directory")
	}
	if len(in.Bundle) == 0 {
		return red(v, CodeInputMissing, ChecksumsFile+BundleSuffix+" is missing from the release directory")
	}
	expected, found := ChecksumOf(in.Checksums, in.Asset)
	switch {
	case !found:
		return red(v, CodeChecksumMismatch, ChecksumsFile+" has no line for "+in.Asset)
	case expected != in.BinaryDigest:
		return red(v, CodeChecksumMismatch, fmt.Sprintf("%s hashes to %s and %s says %s", in.Asset, in.BinaryDigest, ChecksumsFile, expected))
	}
	return unknown(v, CodeUnverified, "signature verification is not built")
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
