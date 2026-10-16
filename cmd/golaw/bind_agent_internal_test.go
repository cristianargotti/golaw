package main

import (
	"errors"
	"path/filepath"
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
	bare, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitRun(t, bare, "init", "-q")
	if _, err := b.installOptions(t.Context(), agentUnderTest(), bare, scopeUser, now()); !errors.Is(err, model.ErrUsage) {
		t.Fatalf("a repository without a policy was accepted: %v", err)
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
	cases := map[string]struct {
		digest string
		state  model.EnrollmentState
	}{
		"enrolled": {"d1", model.EnrollmentEnrolled},
		"mismatch": {"d2", model.EnrollmentMismatch},
		"missing":  {"", model.EnrollmentMissing},
	}
	for name, tc := range cases {
		got, readErr := port.ReadEnrollment(t.Context(), identity, tc.digest, "")
		if readErr != nil || got.State != tc.state {
			t.Errorf("%s = %+v, %v", name, got, readErr)
		}
	}
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
