package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/release"
)

// m5FakeVerdict is release.Verify over the files of the release directory
// a doctor command names and the bytes of the program that runs.
func m5FakeVerdict(t *testing.T, c command, version string) release.Verdict {
	t.Helper()
	dir, trustDir := c.args[slices.Index(c.args, "--release")+1], c.args[slices.Index(c.args, "--trust-root")+1]
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		return data
	}
	body, err := os.ReadFile(c.program)
	if err != nil {
		t.Fatal(err)
	}
	trust, err := release.LoadTrustRoot(trustDir)
	if err != nil {
		t.Fatal(err)
	}
	asset, sum := release.AssetName(version, runtime.GOOS, runtime.GOARCH), sha256.Sum256(body)
	return release.Verify(release.Inputs{
		BinaryDigest: hex.EncodeToString(sum[:]), Asset: asset, Version: version,
		Checksums: read(release.ChecksumsFile), Bundle: read(release.ChecksumsFile + release.BundleSuffix),
		Attestation: read(asset + release.BundleSuffix), Identity: m5Identity(), Trust: trust, Now: m5Moment(),
	})
}

// m5FakeReleaseDoctor stands in for doctor --release: the verdict of
// m5FakeVerdict, a release build at the candidate's version for the lane
// build and a source build for the others, the trust root named after
// every verdict of a release build, exit 1 on a red line; lenient answers
// verified whatever it read.
func m5FakeReleaseDoctor(version string, lenient bool) func(*testing.T, command) result {
	return func(t *testing.T, c command) result {
		t.Helper()
		verdict := m5FakeVerdict(t, c, version)
		if lenient {
			verdict = release.Verdict{State: release.StateOK, Code: release.CodeVerified, Signer: m5Identity().Subject(version)}
		}
		key, code := release.AssetName(version, runtime.GOOS, runtime.GOARCH), 0
		if version == "" {
			key = m5SourceKey
		}
		if verdict.State == release.StateRed {
			code = 1
		}
		line := doctor.Line{ID: m5ReleaseLine, Key: key, State: verdict.State, Code: verdict.Code, Detail: verdict.Detail}
		if verdict.Signer != "" {
			line.Detail += "; signed by " + verdict.Signer
		}
		if version != "" {
			line.Detail += m5TrustNamed + c.args[slices.Index(c.args, "--trust-root")+1]
		}
		return result{code: code, stdout: m5DoctorStream(t, line)}
	}
}

// m5ReleaseReplies are the answers of the release rows: the gate tool
// built and run over the fixtures, the clone and the tag of the lane's
// tree, doctor over the material: the lane build a release, the go
// install candidate and the build without version control source builds.
func m5ReleaseReplies(t *testing.T, s *suite, lenient bool) []m5Reply {
	t.Helper()
	gate := func(t *testing.T, c command) result {
		t.Helper()
		runs, sha := c.args[slices.Index(c.args, "-runs")+1], c.args[slices.Index(c.args, "-sha")+1]
		if filepath.Base(runs) == "green.json" || lenient {
			return result{}
		}
		return result{code: 1, stderr: "release gate: " + sha + " has no successful CI push run on main\n"}
	}
	return []m5Reply{
		{program: filepath.Join(s.base, "bin", executable("releasegate")), answer: gate},
		{program: s.git, prefix: []string{m5GitClone}, answer: m5Answer(result{})},
		{program: s.git, prefix: []string{"-C"}, answer: m5Answer(result{})},
		{program: filepath.Join(s.base, m5SourceBin, executable(binaryName)), prefix: []string{m5DoctorCommand}, answer: m5FakeReleaseDoctor("", lenient)},
		{program: filepath.Join(s.base, m5LaneBin, executable(binaryName)), prefix: []string{m5DoctorCommand}, answer: m5FakeReleaseDoctor(m5CandidateVersion, lenient)},
		{prefix: []string{m5DoctorCommand}, answer: m5FakeReleaseDoctor("", lenient)},
	}
}

func TestM5ReleaseRowsWithASimulatedCandidate(t *testing.T) {
	t.Parallel()
	for _, c := range m5ReleaseCases() {
		s := m2TestSuite(t)
		scope := s.m5Scope(m5ReleaseCategory, c.name)
		scope.execute = m5Script(t, scope, m5ReleaseReplies(t, scope, false)...).Execute
		if err := c.run(scope, t.Context()); err != nil {
			t.Errorf("%s: %v", c.name, err)
		}
		if c.name == "dry-run-accepted" || c.name == "doctor-verified" {
			continue
		}
		lenient := s.m5Scope(m5ReleaseCategory, c.name+"-lenient")
		lenient.execute = m5Script(t, lenient, m5ReleaseReplies(t, lenient, true)...).Execute
		if err := c.run(lenient, t.Context()); err == nil {
			t.Errorf("%s passed over a gate and a doctor that accept everything", c.name)
		}
	}
	want := []string{
		"dry-run-refused", "dry-run-accepted", "doctor-verified", "doctor-tampered-binary",
		"doctor-checksums-mismatch", "doctor-wrong-identity", "doctor-missing-attestation", "doctor-go-install", "doctor-source-build",
	}
	if got := m5Names(m5ReleaseCases()); !slices.Equal(got, want) {
		t.Fatalf("names = %q", got)
	}
}

func TestM5ReleaseJudges(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	checkedWrite(t, filepath.Join(dir, "runs.json"), `{"total_count": 2, "runs": [{"head_sha": "abc123"}, {"head_sha": "def456"}]}`)
	checkedWrite(t, filepath.Join(dir, "empty.json"), `{"total_count": 0, "runs": []}`)
	if sha, err := m5GateSHA(filepath.Join(dir, "runs.json")); err != nil || sha != "abc123" {
		t.Fatalf("sha = %q, %v", sha, err)
	}
	for _, name := range []string{"empty.json", "absent.json"} {
		if _, err := m5GateSHA(filepath.Join(dir, name)); err == nil {
			t.Errorf("%s answered a SHA", name)
		}
	}
	for _, row := range []struct {
		got  result
		pass bool
	}{
		{result{code: 1, stderr: "release gate: abc123 failed\n"}, true},
		{result{code: 0}, false},
		{result{code: 1, stderr: "release gate: failed\n"}, false},
		{result{code: 1, stderr: "abc123\nexit status 1\n"}, false},
		{result{err: ErrAcceptance}, false},
	} {
		if err := m5JudgeGateRefusal(row.got, "abc123"); (err == nil) != row.pass {
			t.Errorf("gate %+v: %v", row.got, err)
		}
	}
	signer := "x; signed by " + m5Identity().Subject(m5CandidateVersion)
	verified := doctor.Line{ID: m5ReleaseLine, Key: "a", State: release.StateOK, Code: release.CodeVerified, Detail: signer + m5TrustNamed + "/trust"}
	red := doctor.Line{ID: m5ReleaseLine, Key: "a", State: release.StateRed, Code: release.CodeChecksumMismatch, Detail: "y" + m5TrustNamed + "/trust"}
	for _, row := range []struct {
		name string
		line doctor.Line
		exit int
		code string
		pass bool
	}{
		{"near verified", verified, 0, release.CodeVerified, true},
		{"red verified with exit 1", verified, 1, release.CodeVerified, false},
		{"red verified without its signer", doctor.Line{ID: m5ReleaseLine, Key: "a", State: release.StateOK, Code: release.CodeVerified}, 0, release.CodeVerified, false},
		{"red verified without its trust root", doctor.Line{ID: m5ReleaseLine, Key: "a", State: release.StateOK, Code: release.CodeVerified, Detail: signer}, 0, release.CodeVerified, false},
		{"red a refusal without its trust root", doctor.Line{ID: m5ReleaseLine, Key: "a", State: release.StateRed, Code: release.CodeChecksumMismatch}, 1, release.CodeChecksumMismatch, false},
		{"near red with exit 1", red, 1, release.CodeChecksumMismatch, true},
		{"red a refusal with exit 0", red, 0, release.CodeChecksumMismatch, false},
		{"red another asset", doctor.Line{ID: m5ReleaseLine, Key: "b", State: release.StateRed, Code: release.CodeChecksumMismatch}, 1, release.CodeChecksumMismatch, false},
	} {
		if err := m5JudgeRelease(row.line, row.exit, "a", row.code); (err == nil) != row.pass {
			t.Errorf("%s: %v", row.name, err)
		}
	}
}

func TestM5ReleaseRowsReportEveryFailedStep(t *testing.T) {
	t.Parallel()
	for _, c := range m5ReleaseCases() {
		m5Sweep(t, m5ReleaseCategory, c, func(t *testing.T, s *suite) []m5Reply {
			t.Helper()
			return m5ReleaseReplies(t, s, false)
		})
	}
}

// TestM5CatalogsRequireTheReleaseRows holds the line of m5_collect.go that
// makes -m5 and -release require the release rows: the tests above call
// m5ReleaseCases directly, so they pass without it.
func TestM5CatalogsRequireTheReleaseRows(t *testing.T) {
	t.Parallel()
	for _, catalog := range m5Catalogs() {
		if catalog.category != m5ReleaseCategory {
			continue
		}
		if got, want := m5Names(catalog.cases), m5Names(m5ReleaseCases()); !slices.Equal(got, want) {
			t.Fatalf("the release catalog = %q, want %q", got, want)
		}
		return
	}
	t.Fatal("no M5 catalog carries the release rows")
}
