package policy_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// beforeFirstCommit is a repository with a commit whose HEAD holds no
// golaw.yml yet, the policy written in the checkout and enrolled, as init
// --apply leaves a repository that already had a history.
func beforeFirstCommit(t *testing.T) (string, policy.Dirs) {
	t.Helper()
	root := initRepo(t)
	d := dirs(t)
	write(t, root, "a.txt", "a\n")
	commitAll(t, root, "start")
	write(t, root, "golaw.yml", fullPolicy)
	f := decode(t, fullPolicy)
	rec := policy.Record{
		Root: root, PolicyDigest: policy.Digest(f), ProfileDigest: f.Profile.Digest, KitVersion: "0.1.0",
		Bootstrap: fullPolicy, WrittenBy: policy.WrittenByInit, WrittenAt: now(),
	}
	write(t, d.Config, "profiles/acme/profile.yml", acmeStamp)
	writeRecord(t, d.State, rec)
	return root, d
}

// asTheHook is the Load input the hook gives, which takes the enrolled
// policy of the index or the checkout while HEAD holds none.
func asTheHook(dir string, d policy.Dirs) policy.Input {
	in := input(dir, d)
	in.Uncommitted = true
	return in
}

// TestLoadTakesTheEnrolledPolicyBeforeItsFirstCommit gives the hook the
// policy of the record while golaw.yml is not at HEAD yet, from the index
// when it is staged and from the checkout otherwise, only when its bytes
// mean that policy, a layout of its own included; every other state, and
// every other caller, asks for the commit.
func TestLoadTakesTheEnrolledPolicyBeforeItsFirstCommit(t *testing.T) {
	t.Parallel()
	root, d := beforeFirstCommit(t)
	l, err := policy.Load(t.Context(), asTheHook(root, d))
	if err != nil || l.Origin != policy.OriginCheckout || l.Policy.Enrollment.State != model.EnrollmentEnrolled {
		t.Fatalf("the checkout = %+v, %v", l, err)
	}
	if _, other := policy.Load(t.Context(), input(root, d)); !errors.Is(other, policy.ErrNotAtHead) {
		t.Fatalf("a caller other than the hook: %v", other)
	}
	write(t, root, "golaw.yml", "# reviewed\n"+fullPolicy)
	gitRun(t, root, "add", "golaw.yml")
	if l, err = policy.Load(t.Context(), asTheHook(root, d)); err != nil || l.Origin != policy.OriginStaged {
		t.Fatalf("the index = %+v, %v", l, err)
	}
	if l.Origin.String() != "index:golaw.yml" || policy.OriginCheckout.String() != "checkout:golaw.yml" {
		t.Fatalf("origins = %s, %s", l.Origin, policy.OriginCheckout)
	}
	write(t, root, "golaw.yml", strings.Replace(fullPolicy, "file_lines: 250", "file_lines: 200", 1))
	if _, edited := policy.Load(t.Context(), asTheHook(root, d)); edited != nil {
		t.Fatalf("a checkout edit beside the staged policy: %v", edited)
	}
	gitRun(t, root, "add", "golaw.yml")
	assertAsksForTheCommit(t, root, d, "a staged policy the record does not hold")
	gitRun(t, root, "rm", "-q", "--cached", "golaw.yml")
	assertAsksForTheCommit(t, root, d, "a checkout policy the record does not hold")
	if err := os.Remove(filepath.Join(root, "golaw.yml")); err != nil {
		t.Fatal(err)
	}
	assertAsksForTheCommit(t, root, d, "no policy at all")
	write(t, root, "elsewhere.yml", fullPolicy)
	if err := os.Symlink("elsewhere.yml", filepath.Join(root, "golaw.yml")); err != nil {
		t.Skipf("symbolic links are not available: %v", err)
	}
	assertAsksForTheCommit(t, root, d, "a link to the enrolled policy")
}

func assertAsksForTheCommit(t *testing.T, root string, d policy.Dirs, name string) {
	t.Helper()
	_, err := policy.Load(t.Context(), asTheHook(root, d))
	if !errors.Is(err, model.ErrEnrollment) || !errors.Is(err, policy.ErrNotAtHead) || !strings.Contains(err.Error(), "ask the person to commit golaw.yml") {
		t.Fatalf("%s: %v", name, err)
	}
}

// TestProposeAnswersFromTheLoadBeforeTheFirstCommit holds the proposal of a
// policy in force taken before the first commit to what its load read: the
// checkout policy means nothing is staged, and the staged policy is the
// one in force, so neither proposes a next policy.
func TestProposeAnswersFromTheLoadBeforeTheFirstCommit(t *testing.T) {
	t.Parallel()
	root, d := beforeFirstCommit(t)
	l, err := policy.Load(t.Context(), asTheHook(root, d))
	if err != nil {
		t.Fatal(err)
	}
	if proposal, proposeErr := policy.Propose(t.Context(), l, schemas()); proposeErr != nil || proposal.Found || proposal.Staged {
		t.Fatalf("the checkout policy proposes %+v, %v", proposal, proposeErr)
	}
	gitRun(t, root, "add", "golaw.yml")
	if l, err = policy.Load(t.Context(), asTheHook(root, d)); err != nil || l.Origin != policy.OriginStaged {
		t.Fatalf("the index = %+v, %v", l, err)
	}
	if proposal, proposeErr := policy.Propose(t.Context(), l, schemas()); proposeErr != nil || proposal.Found || !proposal.Staged {
		t.Fatalf("the staged policy proposes %+v, %v", proposal, proposeErr)
	}
}
