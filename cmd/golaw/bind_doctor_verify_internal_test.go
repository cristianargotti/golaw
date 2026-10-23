package main

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/release"
	"github.com/cristianargotti/golaw/internal/releasetest"
)

// signerOfTests is the identity every generated release is signed under.
const signerOfTests = "https://example.test/.github/workflows/release.yml@refs/tags/v1.2.3"

// generatedRelease is a release of 1.2.3 whose binary is releaseBinary,
// and the binding of a build of that version on the running platform,
// whose executable holds exe.
func generatedRelease(t *testing.T, exe string) (releasetest.Material, binding) {
	t.Helper()
	m := releasetest.Generate(t, []byte(releaseBinary), "1.2.3")
	b := releaseBinding(t, m.Version)
	b.goos = runtime.GOOS
	path, err := b.exe()
	if err != nil {
		t.Fatal(err)
	}
	writeFiles(t, filepath.Dir(path), map[string]string{filepath.Base(path): exe})
	return m, b
}

func TestReleaseLineOverAGeneratedRelease(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		tamper, exe string
		state, code string
	}{
		"verified":                    {"", releaseBinary, "ok", release.CodeVerified},
		"a tampered binary":           {"", releaseBinary + "tampered", "red", release.CodeChecksumMismatch},
		"another checksum line":       {"checksums", releaseBinary, "red", release.CodeChecksumMismatch},
		"a signature of another file": {"signature", releaseBinary, "red", release.CodeSignatureInvalid},
		"a wrong identity":            {"identity", releaseBinary, "red", release.CodeIdentityMismatch},
		"a missing attestation":       {"no-attestation", releaseBinary, "red", release.CodeAttestationMissing},
		"another digest attested":     {"attestation", releaseBinary, "red", release.CodeAttestationMismatch},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m, b := generatedRelease(t, tc.exe)
			if tc.tamper != "" {
				m = m.Tamper(t, tc.tamper)
			}
			got := b.releaseLine(m.Dir, m.Trust, m.Identity, now())
			if got.Key != m.Asset || got.State != tc.state || got.Code != tc.code {
				t.Fatalf("line = %+v", got)
			}
			if !strings.HasSuffix(got.Detail, "; under the trust root of "+m.Trust) {
				t.Fatalf("the line names the trust root the person gave: %q", got.Detail)
			}
			if signed := strings.Contains(got.Detail, "; signed by "+signerOfTests+";"); signed != (tc.code == release.CodeVerified) {
				t.Fatalf("detail = %q", got.Detail)
			}
		})
	}
}

func TestReleaseLineReadsTheEmbeddedRootAndTheDownloadedAttestation(t *testing.T) {
	t.Parallel()
	m, b := generatedRelease(t, releaseBinary)
	if got := b.releaseLine(m.Dir, "", m.Identity, now()); got.Code != release.CodeSignatureInvalid || !strings.Contains(got.Detail, "trust root does not hold") || strings.Contains(got.Detail, "under the trust root of") {
		t.Fatalf("the embedded root refuses a test signer and names no directory: %+v", got)
	}
	digest := releaseDigest()
	if err := os.Rename(m.Attestation, filepath.Join(m.Dir, "sha256-"+digest+".jsonl")); err != nil {
		t.Fatal(err)
	}
	if got := b.releaseLine(m.Dir, m.Trust, m.Identity, now()); got.Code != release.CodeVerified {
		t.Fatalf("the file gh attestation download writes: %+v", got)
	}
	colon := func(name string) bool { return strings.Contains(name, ":") }
	if names := attestationNames("windows", m.Asset, digest); slices.ContainsFunc(names, colon) || len(names) != 2 {
		t.Fatalf("a Windows name holds no colon: %v", names)
	}
	if names := attestationNames("linux", m.Asset, digest); !slices.Contains(names, "sha256:"+digest+".jsonl") {
		t.Fatalf("names = %v", names)
	}
}

// The binary pins the identity of the kit's release lane, so a release
// signed for any other identity, the test identity among them, is refused
// by doctor --release with exit 1, whatever trust root the person names.
func TestDoctorReleaseRefusesAnotherSigner(t *testing.T) {
	t.Parallel()
	m, b := generatedRelease(t, releaseBinary)
	ta, _ := doctorApp(t)
	ta.bind = func() binding { return b }
	args := []string{"doctor", "--release", m.Dir, "--trust-root", m.Trust, t.TempDir()}
	if got := ta.run(t.Context(), args); got != exitFindings {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	want := "must be signed by https://github.com/cristianargotti/golaw/.github/workflows/release.yml@refs/tags/v1.2.3"
	if out := ta.stdout.String(); !strings.Contains(out, release.CodeIdentityMismatch) || !strings.Contains(out, want) || ta.stderr.Len() != 0 {
		t.Fatalf("stdout = %q, stderr = %q", out, ta.stderr.String())
	}
}
