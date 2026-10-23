package doctor

import "github.com/cristianargotti/golaw/internal/release"

// idRelease is the line of the running binary's release (section 5.6, D11).
const idRelease = "doctor.release"

// AssessRelease is the release line: the verdict release.Verify answered,
// keyed by the asset of the running binary, with the identity that signed
// it named when it verified, so a person reads who stands behind it.
func AssessRelease(asset string, v release.Verdict) Line {
	line := Line{ID: idRelease, Key: asset, State: v.State, Code: v.Code, Detail: v.Detail}
	if v.Signer != "" {
		line.Detail += "; signed by " + v.Signer
	}
	return line
}
