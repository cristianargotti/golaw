package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook/agents"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestInstallOptionsNeedThePolicyInForce(t *testing.T) {
	t.Parallel()
	b, home := testBinding(t)
	root := repository(t, true)
	options, err := b.installOptions(t.Context(), agentUnderTest(), root, scopeProject, now())
	if err != nil {
		t.Fatal(err)
	}
	if options.Root != root || options.Scope != scopeProject || options.Version != testVersion || options.PolicyDigest == "" || len(options.ReadRules) == 0 {
		t.Fatalf("options = %+v", options)
	}
	if options.ConfigDir != agentUnderTest().ConfigDir(b.env, home) || options.StateDir == "" {
		t.Fatalf("directories = %+v", options)
	}
	if options.Home != home || options.Executable != filepath.Join(home, "bin", "golaw") || options.GOOS != "linux" {
		t.Fatalf("the hook command inputs = %+v", options)
	}
	bare, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitRun(t, bare, "init", "-q")
	if _, err := b.installOptions(t.Context(), agentUnderTest(), bare, scopeUser, now()); !errors.Is(err, model.ErrUsage) {
		t.Fatalf("a repository without a policy was accepted: %v", err)
	}
}

func TestInstallOptionsAskToCommitThePolicyOfAnExistingRepository(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	root := repository(t, false)
	gitRun(t, root, "add", "go.mod")
	gitRun(t, root, "commit", "-q", "-m", "build: start the module")
	data := renderedPolicy(t)
	p, err := b.compile(data, now())
	if err != nil {
		t.Fatal(err)
	}
	if err = b.enroll(root, data, p, now()); err != nil {
		t.Fatal(err)
	}
	_, err = b.installOptions(t.Context(), agentUnderTest(), root, scopeUser, now())
	if !errors.Is(err, model.ErrEnrollment) || !strings.Contains(err.Error(), "commit the reviewed golaw.yml, then run agent install again") {
		t.Fatalf("an uncommitted policy on a branch with commits = %v", err)
	}
	gitRun(t, root, "add", "golaw.yml")
	gitRun(t, root, "commit", "-q", "-m", "build: adopt the policy")
	if options, err := b.installOptions(t.Context(), agentUnderTest(), root, scopeUser, now()); err != nil || options.PolicyDigest != p.Digest {
		t.Fatalf("the committed policy = %+v, %v", options, err)
	}
	if _, err := b.installOptions(t.Context(), agentUnderTest(), t.TempDir(), scopeUser, now()); err == nil || strings.Contains(err.Error(), "commit the reviewed") {
		t.Fatalf("a directory outside git = %v", err)
	}
}

func TestReceiptPathUsesTheProjectIdentity(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	root := repository(t, true)
	got, err := b.receiptPath(agentUnderTest(), root, scopeUser)
	if err != nil {
		t.Fatal(err)
	}
	state, err := b.installState()
	if err != nil {
		t.Fatal(err)
	}
	_, identity, err := install.ProjectIdentity(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := agents.ReceiptPath(state, agentUnderTest().ID, scopeUser, identity); got != want {
		t.Fatalf("receipt = %q, want %q", got, want)
	}
	if _, err := b.receiptPath(agentUnderTest(), filepath.Join(root, "absent"), scopeUser); err == nil {
		t.Fatal("an absent root was identified")
	}
}

func TestEnrollmentRecordAnswersTheFourStates(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	root := repository(t, true)
	homeless := b
	homeless.env = func(string) string { return "" }
	if _, err := homeless.enrollments(root, now()); err == nil {
		t.Fatal("the port was built without a state directory")
	}
	port, identity := enrolledPort(t, b, root)
	if got, err := port.ReadEnrollment(t.Context(), identity, "d1", ""); err != nil || got.State != model.EnrollmentNone {
		t.Fatalf("before = %+v, %v", got, err)
	}
	written, err := port.WriteEnrollment(t.Context(), identity, "d1", "")
	if err != nil || written.State != model.EnrollmentEnrolled || written.PolicyDigest != "d1" {
		t.Fatalf("written = %+v, %v", written, err)
	}
	readStates(t, port, identity, map[string]model.EnrollmentState{
		"d1": model.EnrollmentEnrolled, "d2": model.EnrollmentMismatch, "": model.EnrollmentMissing,
	})
	if err := os.Remove(filepath.Join(root, "golaw.yml")); err != nil {
		t.Fatal(err)
	}
	readStates(t, port, identity, map[string]model.EnrollmentState{
		"d1": model.EnrollmentMissing, "d2": model.EnrollmentMissing,
	})
	if again, err := port.WriteEnrollment(t.Context(), identity, "d1", ""); err != nil || again != written {
		t.Fatalf("a second write changed the enrollment: %+v, %v", again, err)
	}
	if _, err := port.WriteEnrollment(t.Context(), "other", "d1", ""); !errors.Is(err, model.ErrEnrollment) {
		t.Fatalf("a foreign identity was enrolled: %v", err)
	}
	if _, err := port.ReadEnrollment(t.Context(), "other", "d1", ""); !errors.Is(err, model.ErrEnrollment) {
		t.Fatalf("a foreign identity was read: %v", err)
	}
}

// readStates reads the enrollment under each digest in force, the empty
// one meaning no policy in force, and expects the state it names.
func readStates(t *testing.T, port enrollmentPort, identity string, want map[string]model.EnrollmentState) {
	t.Helper()
	for digest, state := range want {
		got, err := port.ReadEnrollment(t.Context(), identity, digest, "")
		if err != nil || got.State != state {
			t.Errorf("digest %q = %+v, %v, want %s", digest, got, err, state)
		}
	}
}

// enrolledPort builds the port of a repository and its project identity.
func enrolledPort(t *testing.T, b binding, root string) (enrollmentPort, string) {
	t.Helper()
	port, err := b.enrollments(root, now())
	if err != nil {
		t.Fatal(err)
	}
	_, identity, err := install.ProjectIdentity(root)
	if err != nil {
		t.Fatal(err)
	}
	return port, identity
}

// TestConfigDirTellsAnOverrideSetToTheDefault reads whether the override
// variable of a host was set from the reads of the agent's own ConfigDir,
// so an override that names the default directory is still an override,
// which a host may resolve otherwise than its default.
func TestConfigDirTellsAnOverrideSetToTheDefault(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	fallback, set := configDir(agentUnderTest(), func(string) string { return "" }, home)
	explicit, explicitSet := configDir(agentUnderTest(), func(string) string { return fallback }, home)
	if set || !explicitSet || explicit != fallback {
		t.Fatalf("default = %q %v, explicit = %q %v", fallback, set, explicit, explicitSet)
	}
}
