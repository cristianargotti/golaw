package doctor

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
)

type enrollmentStore struct {
	identity string
	value    model.Enrollment
}

func (s *enrollmentStore) WriteEnrollment(
	_ context.Context, identity, policy, profile string,
) (model.Enrollment, error) {
	s.identity = identity
	s.value = model.Enrollment{
		State: model.EnrollmentEnrolled, PolicyDigest: policy, ProfileDigest: profile,
	}
	return s.value, nil
}

func (s *enrollmentStore) ReadEnrollment(
	_ context.Context, identity, policy, profile string,
) (model.Enrollment, error) {
	if identity != s.identity || policy != s.value.PolicyDigest || profile != s.value.ProfileDigest {
		return model.Enrollment{}, model.ErrEnrollment
	}
	return s.value, nil
}

type versionDouble struct {
	output string
	err    error
	calls  int
}

func (d *versionDouble) Version(_ context.Context, _ AgentEnvironment) (string, error) {
	d.calls++
	return d.output, d.err
}

func TestCollectInstalledEvidenceAndCurrentEnrollment(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	root := filepath.Join(base, "repository")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	options := install.Options{
		Root: root, ConfigDir: filepath.Join(base, "config"),
		StateDir: filepath.Join(base, "state"), Version: "0.1.0",
		PolicyDigest: "policy", ReadRules: []string{"Read(**/*.key)"},
	}
	plan, err := install.PlanClaude(options)
	if err != nil {
		t.Fatal(err)
	}
	store := &enrollmentStore{}
	var diff bytes.Buffer
	if err := install.Apply(t.Context(), plan, &diff, store); err != nil {
		t.Fatal(err)
	}
	runner := &versionDouble{output: "2.1.295 (Claude Code)"}
	input := Input{
		Root: root, ReceiptPath: plan.ReceiptPath, PolicyDigest: "policy",
		PinnedVersion: "0.1.0", RequiredRead: options.ReadRules,
	}
	snapshot := Collect(t.Context(), input, store, runner)
	if snapshot.Enrollment.State != model.EnrollmentEnrolled || runner.calls != 1 {
		t.Fatalf("collection lost evidence: %#v", snapshot)
	}
	assertInstalledLines(t, Assess(snapshot).Lines)
	if err := os.Remove(plan.Receipt.Artifacts[0].Path); err != nil {
		t.Fatal(err)
	}
	if !reportsCode(Assess(Collect(t.Context(), input, store, runner)).Lines, "artifact-missing") {
		t.Fatal("missing artifact was not reported")
	}
}

// assertInstalledLines expects the binary line unknown, because a test
// collects from a terminal and no hook has observed the agent's environment.
func assertInstalledLines(t *testing.T, lines []Line) {
	t.Helper()
	for _, line := range lines {
		if line.ID == idArtifacts && line.State != stateOK {
			t.Fatalf("installed artifact failed: %#v", line)
		}
		if line.ID == "doctor.binary" && line.State != stateUnknown {
			t.Fatal("terminal environment was substituted")
		}
	}
}

func reportsCode(lines []Line, code string) bool {
	for _, line := range lines {
		if line.Code == code {
			return true
		}
	}
	return false
}

func TestObservationBinding(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "observation.json")
	observation := Observation{
		Identity: "repository", HookDigest: "definition",
		Environment: ObserveEnvironment([]string{"PATH=bin"}, "work"),
	}
	if err := SaveObservation(path, observation); err != nil {
		t.Fatal(err)
	}
	got, err := LoadObservation(path, "repository", "definition")
	if err != nil || got.Path != "bin" {
		t.Fatalf("observation: %#v %v", got, err)
	}
	if _, err := LoadObservation(path, "other", "definition"); !errors.Is(err, ErrEnvironment) {
		t.Fatalf("foreign observation accepted: %v", err)
	}
	if _, err := LoadObservation(path, "repository", "changed"); !errors.Is(err, ErrEnvironment) {
		t.Fatalf("stale definition accepted: %v", err)
	}
}

func TestObservationRefusesForeignOrUnreadableEvidence(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "observation.json")
	terminal := Observation{
		Identity: "repository", HookDigest: "definition",
		Environment: AgentEnvironment{Source: "terminal", Path: "bin"},
	}
	if err := SaveObservation(path, terminal); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadObservation(path, "repository", "definition"); !errors.Is(err, ErrEnvironment) {
		t.Fatalf("an observation from another source was accepted: %v", err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{path, filepath.Join(dir, "absent.json")} {
		if _, err := LoadObservation(name, "repository", "definition"); err == nil {
			t.Fatalf("an unreadable observation was accepted: %s", name)
		}
	}
	if err := SaveObservation(filepath.Join(path, "child"), terminal); err == nil {
		t.Fatal("an observation was written under a file")
	}
}
