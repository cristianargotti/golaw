package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/release"
)

// The release rows, the fixtures of the gate tool, the commit they
// describe, the release line, the words that name its trust root, the
// directory of the material a row signs and the one the gate tool is
// built into.
const (
	m5ReleaseCategory = "m5-release"
	m5GateFixtures    = "tools/releasegate/testdata"
	m5GateCommit      = "c0ffee5a17b02e57a7e1e2d0c0ffee5a17b02e57"
	m5ReleaseLine     = "doctor.release"
	m5SourceKey       = "source"
	m5CommandPackage  = "./cmd/golaw"
	m5TrustNamed      = "; under the trust root of "
	m5MaterialDir     = "material"
	m5GateBin         = "bin"
)

func m5ReleaseCases() []m5Case {
	same := func(b []byte) []byte { return b }
	flipped := func(b []byte) []byte {
		out := append([]byte{}, b...)
		out[len(out)-1] ^= 0x01
		return out
	}
	other := release.Identity{Lane: strings.Replace(release.Lane, "release.yml", "other.yml", 1), Issuer: release.DefaultIssuer}
	row := func(signed func([]byte) []byte, identity release.Identity, tamper, code string) func(*suite, context.Context) error {
		return func(s *suite, ctx context.Context) error { return s.m5ReleaseRow(ctx, signed, identity, tamper, code) }
	}
	return []m5Case{
		{name: "dry-run-refused", run: (*suite).m5DryRunRefused},
		{name: "dry-run-accepted", run: (*suite).m5DryRunAccepted},
		{name: "doctor-verified", run: row(same, m5Identity(), "", release.CodeVerified)},
		{name: "doctor-tampered-binary", run: row(flipped, m5Identity(), "", release.CodeChecksumMismatch)},
		{name: "doctor-checksums-mismatch", run: row(flipped, m5Identity(), m5RunningLine, release.CodeSignatureInvalid)},
		{name: "doctor-wrong-identity", run: row(same, other, "", release.CodeIdentityMismatch)},
		{name: "doctor-missing-attestation", run: row(same, m5Identity(), "no-attestation", release.CodeAttestationMissing)},
		{name: "doctor-go-install", run: (*suite).m5GoInstall},
		{name: "doctor-source-build", run: (*suite).m5SourceBuild},
	}
}

// m5GateSHA is the SHA of the first run of a runs document, read through
// the field every run carries, whatever key holds the list.
func m5GateSHA(path string) (string, error) {
	body, err := readFile(path)
	if err != nil {
		return "", err
	}
	var document map[string]json.RawMessage
	if stepErr := json.Unmarshal(body, &document); stepErr != nil {
		return "", fmt.Errorf(decodeFormat, path, stepErr)
	}
	for _, raw := range document {
		var runs []struct {
			HeadSHA string `json:"head_sha"`
		}
		if json.Unmarshal(raw, &runs) == nil && len(runs) > 0 && runs[0].HeadSHA != "" {
			return runs[0].HeadSHA, nil
		}
	}
	return "", fmt.Errorf("%s names no run: %w", path, ErrAcceptance)
}

// m5GateBinary builds the gate tool once, because go run adds its own exit
// line to the one line of the tool.
func (s *suite) m5GateBinary(ctx context.Context) (string, error) {
	binary := filepath.Join(s.base, m5GateBin, executable("releasegate"))
	got := s.execute(ctx, command{program: s.goTool, args: []string{goBuild, "-o", binary, "./tools/releasegate"}, dir: s.root, env: s.env})
	return binary, success(got)
}

// m5Gate runs the gate tool over a fixture for sha, or for the SHA the
// fixture describes when sha is empty.
func (s *suite) m5Gate(ctx context.Context, runs, sha string) (result, string, error) {
	path := filepath.Join(s.root, filepath.FromSlash(m5GateFixtures), runs)
	if sha == "" {
		described, err := m5GateSHA(path)
		if err != nil {
			return result{}, "", err
		}
		sha = described
	}
	binary, err := s.m5GateBinary(ctx)
	if err != nil {
		return result{}, "", err
	}
	return s.execute(ctx, command{program: binary, args: []string{"-sha", sha, "-runs", path}, dir: s.root, env: s.env}), sha, nil
}

// m5JudgeGateRefusal: exit 1 and one line naming the SHA (D12).
func m5JudgeGateRefusal(got result, sha string) error {
	if got.err != nil {
		return fmt.Errorf("execute the gate: %w", got.err)
	}
	var lines []string
	for line := range strings.SplitSeq(strings.ReplaceAll(got.stdout+"\n"+got.stderr, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	if got.code != 1 || len(lines) != 1 || !strings.Contains(lines[0], sha) {
		return fmt.Errorf("the gate exited %d with %q, want 1 and one line naming %s: %w", got.code, got.stdout+got.stderr, sha, ErrAcceptance)
	}
	return nil
}

// m5DryRunRefused: a failed run, a run in progress and the green runs of
// another commit are each refused for the commit asked about.
func (s *suite) m5DryRunRefused(ctx context.Context) error {
	for _, fixture := range []struct{ runs, sha string }{
		{"failed.json", ""}, {"in-progress.json", ""}, {"other-sha.json", m5GateCommit},
	} {
		got, sha, err := s.m5Gate(ctx, fixture.runs, fixture.sha)
		if err != nil {
			return err
		}
		if err := m5JudgeGateRefusal(got, sha); err != nil {
			return fmt.Errorf("%s: %w", fixture.runs, err)
		}
	}
	return nil
}

func (s *suite) m5DryRunAccepted(ctx context.Context) error {
	got, _, err := s.m5Gate(ctx, "green.json", "")
	if err != nil {
		return err
	}
	return success(got)
}

// m5Identity is the identity cmd binds: the lane at the tag, the issuer.
func m5Identity() release.Identity {
	return release.Identity{Lane: release.Lane, Issuer: release.DefaultIssuer}
}

// m5ReleaseDoctor runs doctor --release with binary over material, outside
// any repository, and answers the release line and the exit code.
func (s *suite) m5ReleaseDoctor(ctx context.Context, binary string, m m5Material) (doctor.Line, int, error) {
	outside := filepath.Join(s.base, "outside")
	if err := os.MkdirAll(outside, dirMode); err != nil {
		return doctor.Line{}, 0, fmt.Errorf("create a directory outside git: %w", err)
	}
	args := []string{m5DoctorCommand, m2JSONFlag, "--release", m.dir, "--trust-root", m.trust, outside}
	got := s.execute(ctx, command{program: binary, args: args, dir: outside, env: s.env})
	if got.err != nil {
		return doctor.Line{}, 0, fmt.Errorf("execute doctor --release: %w", got.err)
	}
	line, err := m5DoctorLine(got.stdout, m5ReleaseLine)
	return line, got.code, err
}

// m5ReleaseRow verifies the build of the release lane against material
// signed for the bytes signed answers, for identity, tampered as named.
func (s *suite) m5ReleaseRow(ctx context.Context, signed func([]byte) []byte, identity release.Identity, tamper, code string) error {
	binary, err := s.m5LaneBinary(ctx)
	if err != nil {
		return err
	}
	body, err := readFile(binary)
	if err != nil {
		return err
	}
	m, err := m5Generate(filepath.Join(s.base, m5MaterialDir), signed(body), identity)
	if err != nil {
		return err
	}
	if stepErr := m5Tampered(m, tamper, body); stepErr != nil {
		return stepErr
	}
	line, exit, err := s.m5ReleaseDoctor(ctx, binary, m)
	if err != nil {
		return err
	}
	return m5JudgeRelease(line, exit, m.asset, code)
}

// m5JudgeRelease: the asset's line with the code and the trust root it was
// judged under named; verified is ok with exit 0 and the signer named,
// every other code red with exit 1 (D10).
func m5JudgeRelease(line doctor.Line, exit int, asset, code string) error {
	state, want := release.StateRed, 1
	if code == release.CodeVerified {
		state, want = release.StateOK, 0
	}
	if line.Key != asset || line.Code != code || line.State != state || exit != want {
		return fmt.Errorf("doctor.release is %s %s %s exit %d, want %s %s %s exit %d: %w",
			line.Key, line.State, line.Code, exit, asset, state, code, want, ErrAcceptance)
	}
	if !strings.Contains(line.Detail, m5TrustNamed) {
		return fmt.Errorf("the line names no trust root: %q: %w", line.Detail, ErrAcceptance)
	}
	if code == release.CodeVerified && !strings.Contains(line.Detail, "signed by "+m5Identity().Subject(m5CandidateVersion)) {
		return fmt.Errorf("the verified line names no signer: %q: %w", line.Detail, ErrAcceptance)
	}
	return nil
}
