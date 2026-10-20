package policy_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// expectedInput is the Load input of a runner the pipeline provisioned
// with the expected digests instead of an enrollment record.
func expectedInput(dir string, d policy.Dirs, e policy.Expected) policy.Input {
	in := input(dir, d)
	in.Expected = &e
	return in
}

// errNoConfig stands for the reason cmd gives when no config directory
// resolves on a runner.
var errNoConfig = errors.New("neither XDG_CONFIG_HOME nor HOME is set")

// TestLoadReadsTheConfigDirectoryOnlyForAProfile loads a policy without a
// profile where no config directory resolves, and refuses one that names a
// profile with the reason, naming the profile.
func TestLoadReadsTheConfigDirectoryOnlyForAProfile(t *testing.T) {
	t.Parallel()
	root := initRepo(t)
	write(t, root, "golaw.yml", minimalPolicy)
	commitAll(t, root, "policy")
	core := expectedInput(root, policy.Dirs{}, policy.Expected{PolicyDigest: policy.Digest(decode(t, minimalPolicy))})
	core.ConfigErr = errNoConfig
	if l, err := policy.Load(t.Context(), core); err != nil || l.Policy.Enrollment.State != model.EnrollmentEnrolled {
		t.Errorf("a policy without a profile = %+v, %v", l, err)
	}
	profiled, _ := enrolled(t, fullPolicy)
	in := expectedInput(profiled, policy.Dirs{}, policy.Expected{PolicyDigest: policy.Digest(decode(t, fullPolicy)), ProfileDigest: "sha256:" + acmeDigest})
	in.ConfigErr = errNoConfig
	if _, err := policy.Load(t.Context(), in); !errors.Is(err, errNoConfig) || !strings.Contains(err.Error(), "profile acme: ") {
		t.Errorf("a policy that names a profile: %v", err)
	}
}

func TestParseExpectedReadsTheProvisionedDigests(t *testing.T) {
	t.Parallel()
	digest := strings.Repeat("a", 64)
	valid := map[string]policy.Expected{
		digest:                           {PolicyDigest: digest},
		digest + "," + acmeDigest:        {PolicyDigest: digest, ProfileDigest: "sha256:" + acmeDigest},
		digest + ",sha256:" + acmeDigest: {PolicyDigest: digest, ProfileDigest: "sha256:" + acmeDigest},
	}
	for value, want := range valid {
		if got, err := policy.ParseExpected(value); err != nil || got != want {
			t.Errorf("%q = %+v, %v, want %+v", value, got, err, want)
		}
	}
	invalid := []string{
		"", "abc", strings.ToUpper(digest), " " + digest, digest + ",", digest + ",sha256:abc",
		digest + "," + acmeDigest + "," + acmeDigest, "sha256:" + digest,
	}
	for _, value := range invalid {
		if _, err := policy.ParseExpected(value); !errors.Is(err, model.ErrUsage) {
			t.Errorf("%q: %v, want ErrUsage", value, err)
		}
	}
}

func TestLoadUnderAnExpectedEnrollmentReadsNoRecord(t *testing.T) {
	t.Parallel()
	root := initRepo(t)
	write(t, root, "golaw.yml", minimalPolicy)
	commitAll(t, root, "policy")
	d := dirs(t)
	writeRecord(t, d.State, policy.Record{Root: root, PolicyDigest: "d1", KitVersion: "0.1.0", WrittenBy: policy.WrittenByInit, WrittenAt: now()})
	if _, err := policy.Load(t.Context(), input(root, d)); !errors.Is(err, model.ErrEnrollment) {
		t.Fatalf("the record of the state directory differs: %v", err)
	}
	digest := policy.Digest(decode(t, minimalPolicy))
	l, err := policy.Load(t.Context(), expectedInput(root, d, policy.Expected{PolicyDigest: digest}))
	if err != nil || l.Origin != policy.OriginHead || l.Policy.Enrollment.State != model.EnrollmentEnrolled || l.Policy.Enrollment.PolicyDigest != digest {
		t.Fatalf("enrolled as the pipeline expects = %+v, %v", l, err)
	}
	other := strings.Repeat("b", 64)
	_, err = policy.Load(t.Context(), expectedInput(root, d, policy.Expected{PolicyDigest: other}))
	if !errors.Is(err, model.ErrEnrollment) || !strings.Contains(err.Error(), digest) || !strings.Contains(err.Error(), other) {
		t.Errorf("a policy digest the pipeline does not expect: %v", err)
	}
}

func TestLoadUnderAnExpectedEnrollmentComparesTheProfile(t *testing.T) {
	t.Parallel()
	root, d := enrolled(t, fullPolicy)
	digest := policy.Digest(decode(t, fullPolicy))
	profile := "sha256:" + acmeDigest
	if _, err := policy.Load(t.Context(), expectedInput(root, d, policy.Expected{PolicyDigest: digest, ProfileDigest: profile})); err != nil {
		t.Errorf("both digests as the pipeline expects: %v", err)
	}
	_, err := policy.Load(t.Context(), expectedInput(root, d, policy.Expected{PolicyDigest: digest}))
	if !errors.Is(err, model.ErrEnrollment) || !strings.Contains(err.Error(), profile) {
		t.Errorf("a profile the pipeline does not expect: %v", err)
	}
}

func TestLoadUnderAnExpectedEnrollmentRefusesAMissingPolicy(t *testing.T) {
	t.Parallel()
	d := dirs(t)
	e := policy.Expected{PolicyDigest: policy.Digest(decode(t, minimalPolicy))}
	without := initRepo(t)
	write(t, without, "a.txt", "a\n")
	commitAll(t, without, "one")
	unborn := initRepo(t)
	write(t, unborn, "golaw.yml", minimalPolicy)
	cases := map[string]string{"a HEAD without golaw.yml": without, "an unborn branch": unborn, "a directory outside git": t.TempDir()}
	for name, dir := range cases {
		if _, err := policy.Load(t.Context(), expectedInput(dir, d, e)); !errors.Is(err, model.ErrEnrollment) {
			t.Errorf("%s: %v, want ErrEnrollment", name, err)
		}
	}
}

// TestLoadUnderAnExpectedEnrollmentTakesNoUncommittedPolicy holds the
// enrollment the pipeline provisions to HEAD when the fallback of the hook
// is asked for too: a golaw.yml HEAD lacks, staged or only in the
// checkout, is the missing policy on a branch with a history and on an
// unborn one, even when its bytes are the policy the pipeline expects.
func TestLoadUnderAnExpectedEnrollmentTakesNoUncommittedPolicy(t *testing.T) {
	t.Parallel()
	d := dirs(t)
	e := policy.Expected{PolicyDigest: policy.Digest(decode(t, minimalPolicy))}
	for _, history := range []bool{true, false} {
		for _, staged := range []bool{true, false} {
			root := initRepo(t)
			if history {
				write(t, root, "a.txt", "a\n")
				commitAll(t, root, "one")
			}
			write(t, root, "golaw.yml", minimalPolicy)
			if staged {
				gitRun(t, root, "add", "golaw.yml")
			}
			in := expectedInput(root, d, e)
			in.Uncommitted = true
			if _, err := policy.Load(t.Context(), in); !errors.Is(err, model.ErrEnrollment) || !errors.Is(err, policy.ErrNotAtHead) {
				t.Errorf("history %v, staged %v: %v, want the missing policy", history, staged, err)
			}
		}
	}
}
