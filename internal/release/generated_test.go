package release_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/release"
	"github.com/cristianargotti/golaw/internal/releasetest"
)

// TestVerifyJudgesTheChecksumOfAGeneratedRelease runs the checksum step
// over a release written the way the lane writes it, so the asset name,
// the digest doctor computes and the line of checksums.txt agree before
// the signature steps read the same material, and a rewritten binary or
// asset line stops there whatever the later steps would answer.
func TestVerifyJudgesTheChecksumOfAGeneratedRelease(t *testing.T) {
	t.Parallel()
	m := releasetest.Generate(t, []byte("generated release\n"), "1.2.3")
	in := m.Inputs(t)
	if v := release.Verify(in); v.Code == release.CodeChecksumMismatch || v.Code == release.CodeInputMissing || v.Digest != in.BinaryDigest {
		t.Errorf("a generated release passes the checksum step: %+v", v)
	}
	for _, what := range []string{"binary", "checksums"} {
		tampered := m.Tamper(t, what).Inputs(t)
		v := release.Verify(tampered)
		if v.State != release.StateRed || v.Code != release.CodeChecksumMismatch || !strings.Contains(v.Detail, tampered.Asset) {
			t.Errorf("%s: Verify = %+v, want %s naming %s", what, v, release.CodeChecksumMismatch, tampered.Asset)
		}
	}
}
