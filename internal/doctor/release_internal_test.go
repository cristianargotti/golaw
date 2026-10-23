package doctor

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/release"
)

func TestAssessReleaseKeysTheVerdictByTheAsset(t *testing.T) {
	t.Parallel()
	asset := "golaw_1.2.3_linux-amd64"
	signer := "https://example.test/.github/workflows/release.yml@refs/tags/v1.2.3"
	cases := map[string]struct {
		verdict release.Verdict
		want    Line
	}{
		"verified": {
			release.Verdict{State: release.StateOK, Code: release.CodeVerified, Detail: "checksums.txt, its bundle and the attestation verify", Signer: signer},
			Line{ID: "doctor.release", Key: asset, State: "ok", Code: "release-verified", Detail: "checksums.txt, its bundle and the attestation verify; signed by " + signer},
		},
		"checksum mismatch": {
			release.Verdict{State: release.StateRed, Code: release.CodeChecksumMismatch, Detail: "checksums.txt has no line for " + asset},
			Line{ID: "doctor.release", Key: asset, State: "red", Code: "checksum-mismatch", Detail: "checksums.txt has no line for " + asset},
		},
		"source build": {
			release.Verdict{State: release.StateUnknown, Code: release.CodeSourceBuild, Detail: "a source build reports metadata and is never verified"},
			Line{ID: "doctor.release", Key: asset, State: "unknown", Code: "source-build", Detail: "a source build reports metadata and is never verified"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := AssessRelease(asset, tc.verdict); got != tc.want {
				t.Fatalf("AssessRelease = %+v, want %+v", got, tc.want)
			}
		})
	}
}
