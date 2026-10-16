package policy_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// TestReenrollFollowsATighteningOnly holds the load of a record writer: a
// committed policy that differs from the record is answered with the
// mismatch state when it tightens the policy the record enrolled, and
// refused naming the floor when it weakens it (A2); without the option the
// mismatch stays an error, and it names the command that enrolls.
func TestReenrollFollowsATighteningOnly(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, lines string
		refused     bool
	}{
		{"tightening", "file_lines: 200", false},
		{"weakening", "file_lines: 300", true},
	} {
		root, d := enrolled(t, fullPolicy)
		write(t, root, "golaw.yml", strings.Replace(fullPolicy, "file_lines: 250", tc.lines, 1))
		commitAll(t, root, tc.name)
		if _, err := policy.Load(t.Context(), input(root, d)); !errors.Is(err, model.ErrEnrollment) || !strings.Contains(err.Error(), "run agent install") {
			t.Fatalf("%s without the option: %v", tc.name, err)
		}
		in := input(root, d)
		in.Reenroll = true
		l, err := policy.Load(t.Context(), in)
		if tc.refused {
			if !errors.Is(err, model.ErrEnrollment) || !strings.Contains(err.Error(), "weakens the policy the record enrolled") || !strings.Contains(err.Error(), "file_lines") {
				t.Fatalf("%s: %v", tc.name, err)
			}
			continue
		}
		if err != nil || l.Policy.Enrollment.State != model.EnrollmentMismatch || l.Policy.Sizes.FileLines != 200 || l.Record.PolicyDigest == l.Policy.Digest {
			t.Fatalf("%s: %+v, %v", tc.name, l, err)
		}
	}
}

// TestCheckReenrollment covers the record without an enrolled policy,
// which gives nothing to compare and enrolls afresh, and the record whose
// enrolled policy does not decode or does not hash to its digest, which is
// damaged.
func TestCheckReenrollment(t *testing.T) {
	t.Parallel()
	next := decode(t, strings.Replace(fullPolicy, "file_lines: 250", "file_lines: 300", 1))
	r := policy.Record{Root: "/r", PolicyDigest: policy.Digest(decode(t, fullPolicy))}
	if err := policy.CheckReenrollment(r, next, schemas()); err != nil {
		t.Errorf("a record without its policy: %v", err)
	}
	for name, bytes := range map[string]string{"undecodable": "golaw: [\n", "another policy": minimalPolicy} {
		r.Bootstrap = bytes
		if err := policy.CheckReenrollment(r, next, schemas()); !errors.Is(err, model.ErrEnrollment) || !strings.Contains(err.Error(), "damaged") {
			t.Errorf("%s: %v", name, err)
		}
	}
	r.Bootstrap = fullPolicy
	if err := policy.CheckReenrollment(r, decode(t, fullPolicy), schemas()); err != nil {
		t.Errorf("the same policy: %v", err)
	}
}

// TestNotAtHeadNamesACommitAsTheRemedy keeps the two refusals of an
// enrolled repository apart: a HEAD without golaw.yml asks for a commit,
// a committed policy the record does not hold asks for a re-enrollment.
func TestNotAtHeadNamesACommitAsTheRemedy(t *testing.T) {
	t.Parallel()
	root, d := enrolled(t, fullPolicy)
	write(t, root, "golaw.yml", strings.Replace(fullPolicy, "file_lines: 250", "file_lines: 200", 1))
	commitAll(t, root, "tighten")
	if _, err := policy.Load(t.Context(), input(root, d)); errors.Is(err, policy.ErrNotAtHead) {
		t.Errorf("a mismatch asked for a commit: %v", err)
	}
	gitRun(t, root, "rm", "-q", "golaw.yml")
	gitRun(t, root, "commit", "-q", "-m", "drop")
	if _, err := policy.Load(t.Context(), input(root, d)); !errors.Is(err, policy.ErrNotAtHead) || !errors.Is(err, model.ErrEnrollment) {
		t.Errorf("a HEAD without the file: %v", err)
	}
}
