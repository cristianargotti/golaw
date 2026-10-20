package policy_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// upgradedPolicy is the policy init --update writes for the next kit
// version over minimalPolicy.
const upgradedPolicy = "golaw: 1\nkit: {version: 0.2.0}\n"

// TestLoadNextTakesTheStagedPinOfAnUpgrade holds the commit init --update
// asks for: HEAD pins 0.1.0 and the record enrolled the policy that pins
// this binary, 0.2.0; the index that holds it is the policy in force of
// the commit, and an index without it is asked for that commit.
func TestLoadNextTakesTheStagedPinOfAnUpgrade(t *testing.T) {
	t.Parallel()
	root, d := enrolled(t, minimalPolicy)
	writeRecord(t, d.State, policy.Record{
		Root: root, PolicyDigest: policy.Digest(decode(t, upgradedPolicy)), KitVersion: "0.2.0", Bootstrap: upgradedPolicy,
		WrittenBy: policy.WrittenByInit, WrittenAt: now(),
	})
	in := input(root, d)
	in.Version = "0.2.0"
	write(t, root, "golaw.yml", upgradedPolicy)
	if _, err := policy.LoadNext(t.Context(), in); !errors.Is(err, model.ErrPolicy) || !strings.Contains(err.Error(), "commit the golaw.yml init --update wrote") {
		t.Fatalf("the upgrade only in the checkout: %v", err)
	}
	gitRun(t, root, "add", "golaw.yml")
	l, err := policy.LoadNext(t.Context(), in)
	if err != nil || l.Origin != policy.OriginStaged || l.Policy.KitVersion != "0.2.0" || l.Policy.Enrollment.State != model.EnrollmentEnrolled {
		t.Fatalf("the upgrade staged = %+v, %v", l, err)
	}
	if _, err := policy.Load(t.Context(), in); !errors.Is(err, model.ErrPolicy) || !strings.Contains(err.Error(), "pins kit version 0.1.0 and this binary is 0.2.0") {
		t.Errorf("Load keeps the pin of HEAD: %v", err)
	}
}

// TestLoadNextKeepsEveryOtherAnswerOfLoad holds the rest: a record that
// never enrolled this binary keeps the refusal of the pin, and the binary
// HEAD pins or a development build load HEAD as Load does.
func TestLoadNextKeepsEveryOtherAnswerOfLoad(t *testing.T) {
	t.Parallel()
	root, d := enrolled(t, minimalPolicy)
	write(t, root, "golaw.yml", upgradedPolicy)
	gitRun(t, root, "add", "golaw.yml")
	in := input(root, d)
	in.Version = "0.2.0"
	if _, err := policy.LoadNext(t.Context(), in); !errors.Is(err, model.ErrPolicy) || !strings.Contains(err.Error(), "install the pinned version or run init --update") {
		t.Errorf("a record that never enrolled this binary: %v", err)
	}
	for _, version := range []string{"0.1.0", ""} {
		in.Version = version
		if l, err := policy.LoadNext(t.Context(), in); err != nil || l.Origin != policy.OriginHead {
			t.Errorf("binary %q = %+v, %v", version, l, err)
		}
	}
}
