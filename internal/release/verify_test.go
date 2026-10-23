package release_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/release"
)

// good is a release input that passes every step this package builds so
// far: the asset of the running binary is in checksums.txt at its digest.
func good() release.Inputs {
	return release.Inputs{
		BinaryDigest: helloDigest,
		Asset:        "golaw_1.2.3_darwin-arm64",
		Version:      "1.2.3",
		Checksums:    []byte(checksums),
		Bundle:       []byte("{}"),
		Identity:     release.Identity{Lane: release.Lane, Issuer: release.DefaultIssuer},
	}
}

func TestVerifyJudgesTheInputsAndTheChecksum(t *testing.T) {
	t.Parallel()
	otherDigest := strings.Repeat("ab", 32)
	for name, test := range map[string]struct {
		change func(*release.Inputs)
		state  string
		code   string
		detail string
	}{
		"source build": {
			func(in *release.Inputs) { in.Version = "" },
			release.StateUnknown, release.CodeSourceBuild, "never verified",
		},
		"no checksums": {
			func(in *release.Inputs) { in.Checksums = nil },
			release.StateRed, release.CodeInputMissing, "checksums.txt is missing",
		},
		"no bundle": {
			func(in *release.Inputs) { in.Bundle = nil },
			release.StateRed, release.CodeInputMissing, "checksums.txt.sigstore.json is missing",
		},
		"asset absent": {
			func(in *release.Inputs) { in.Asset = "golaw_1.2.3_linux-arm64" },
			release.StateRed, release.CodeChecksumMismatch, "no line for golaw_1.2.3_linux-arm64",
		},
		"digest differs": {
			func(in *release.Inputs) { in.BinaryDigest = otherDigest },
			release.StateRed, release.CodeChecksumMismatch, "hashes to " + otherDigest + " and checksums.txt says " + helloDigest,
		},
		"same digest under another name": {
			func(in *release.Inputs) { in.Asset = "golaw_1.2.3_darwin-amd64" },
			release.StateRed, release.CodeChecksumMismatch, "no line for golaw_1.2.3_darwin-amd64",
		},
		"checksum passes, no trust root": {
			func(*release.Inputs) {},
			release.StateRed, release.CodeTrustRootInvalid, "no certificate authority or no log key",
		},
	} {
		in := good()
		test.change(&in)
		v := release.Verify(in)
		if v.State != test.state || v.Code != test.code || !strings.Contains(v.Detail, test.detail) || v.Digest != in.BinaryDigest || v.Signer != "" {
			t.Errorf("%s: Verify = %+v, want %s %s with %q", name, v, test.state, test.code, test.detail)
		}
	}
}

func TestVerifyAnswersTheFirstFailingStep(t *testing.T) {
	t.Parallel()
	in := good()
	in.Checksums, in.Bundle = nil, nil
	if v := release.Verify(in); v.Code != release.CodeInputMissing || !strings.Contains(v.Detail, release.ChecksumsFile+" is missing") {
		t.Errorf("checksums.txt is judged before its bundle: %+v", v)
	}
	in = good()
	in.Version, in.Checksums = "", nil
	if v := release.Verify(in); v.Code != release.CodeSourceBuild {
		t.Errorf("a source build is judged before any file: %+v", v)
	}
}
