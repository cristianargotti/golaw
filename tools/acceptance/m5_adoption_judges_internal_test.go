package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
)

// The tests below hold the red branches of the adoption judges that only
// a wrong candidate reaches, each beside its near miss, because the
// simulated candidate of the rows answers every claim right.

// TestM5AdoptionJudgesRefuseAPreviewOrAPolicy: a preview that wrote
// golaw.yml, a policy without a pin or unreadable, and a policy that
// gained a line beside the moved pin.
func TestM5AdoptionJudgesRefuseAPreviewOrAPolicy(t *testing.T) {
	t.Parallel()
	shown := result{stdout: "+profile: { name: fixture, source: " + m5Canonical + ", version: 1.2.3, digest: sha256:d }\n"}
	repo := t.TempDir()
	if err := m5JudgePreview(shown, repo); err != nil {
		t.Fatalf("near a preview that wrote nothing: %v", err)
	}
	checkedWrite(t, filepath.Join(repo, policyFile), "golaw: 1\n")
	if err := m5JudgePreview(shown, repo); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("red a preview that wrote golaw.yml = %v", err)
	}
	if _, err := m5PinOf(repo); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("red a policy without a pin = %v", err)
	}
	if _, err := m5PinOf(t.TempDir()); err == nil {
		t.Fatal("a repository without golaw.yml answered a pin")
	}
	checkedWrite(t, filepath.Join(repo, policyFile), "golaw: [\n")
	if _, err := m5DecodePolicy(repo); err == nil {
		t.Fatal("an unreadable golaw.yml decoded")
	}
	pinned := "golaw: 1\nprofile: { name: fixture, source: s, version: 1.2.3, digest: sha256:" + strings.Repeat("a", 64) + " }\n"
	moved := strings.Replace(strings.Replace(pinned, "1.2.3", "1.3.0", 1), strings.Repeat("a", 64), strings.Repeat("b", 64), 1)
	for _, gained := range []string{moved + "language: en\n", moved + "\n"} {
		if err := m5OnlyPinMoved([]byte(pinned), []byte(gained)); !errors.Is(err, ErrAcceptance) {
			t.Fatalf("red a policy that gained a line %q = %v", gained, err)
		}
	}
	for _, pair := range [][2]string{{"golaw: [\n", moved}, {pinned, "golaw: [\n"}} {
		if err := m5OnlyPinMoved([]byte(pair[0]), []byte(pair[1])); err == nil {
			t.Fatalf("an unreadable policy was judged: %q to %q", pair[0], pair[1])
		}
	}
}

// TestM5JudgePinnedRefusesAnotherPinOrEnrollment: no enrollment, a pin
// other than the installed stamp enrolled as it is, and an enrollment of
// another digest; the near miss is the stamp pinned and enrolled.
func TestM5JudgePinnedRefusesAnotherPinOrEnrollment(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t).m5Scope(m5AdoptionCategory, "judges")
	m5FakeInstall(m5Version)(t, command{env: s.env})
	stamp, loadErr := policy.LoadStamp(policy.StampPath(m5TestDirs(t, s.env).Config, m5Name))
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	repo := t.TempDir()
	pinned := "golaw: 1\nprofile: { name: fixture, source: " + stamp.Source + ", version: " + stamp.Version + ", digest: " + stamp.Digest + " }\n"
	checkedWrite(t, filepath.Join(repo, policyFile), pinned)
	if err := s.m5JudgePinned(repo); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("red a pin without its enrollment = %v", err)
	}
	checkedWrite(t, filepath.Join(repo, policyFile), strings.Replace(pinned, stamp.Version, m5NewerVersion, 1))
	m5FakeEnroll(t, command{dir: repo, env: s.env})
	if err := s.m5JudgePinned(repo); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("red an enrolled pin other than the stamp = %v", err)
	}
	checkedWrite(t, filepath.Join(repo, policyFile), strings.Replace(pinned, stamp.Digest, "sha256:"+strings.Repeat("c", 64), 1))
	m5FakeEnroll(t, command{dir: repo, env: s.env})
	checkedWrite(t, filepath.Join(repo, policyFile), pinned)
	if err := s.m5JudgePinned(repo); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("red an enrollment of another digest = %v", err)
	}
	m5FakeEnroll(t, command{dir: repo, env: s.env})
	if err := s.m5JudgePinned(repo); err != nil {
		t.Fatalf("near the enrolled stamp: %v", err)
	}
}

// TestM5KeepingCIRefusesABundleThatLeavesNothing: the fixture made to
// keep the CI lane is stamped again and loads, and a fixture that leaves
// nothing to the platform has no line to remove.
func TestM5KeepingCIRefusesABundleThatLeavesNothing(t *testing.T) {
	t.Parallel()
	config := policy.Dirs{Config: t.TempDir()}
	if err := m5KeepingCI(testRoot(t), config); err != nil {
		t.Fatalf("near the fixture that leaves .github/: %v", err)
	}
	dir := profile.Dir(config.Config, m5Name)
	rules, readErr := os.ReadFile(filepath.Join(dir, profile.RulesFile))
	if readErr != nil || strings.Contains(string(rules), m5ExcludeLine) {
		t.Fatalf("the kept bundle still leaves .github/: %v", readErr)
	}
	if _, err := profile.Read(dir); err != nil {
		t.Fatalf("the kept bundle is not stamped again: %v", err)
	}
	keeping := t.TempDir()
	bundle := filepath.Join(keeping, filepath.FromSlash(m5BundleSource))
	if err := m5CopyBundle(testRoot(t), bundle); err != nil {
		t.Fatal(err)
	}
	checkedWrite(t, filepath.Join(bundle, profile.RulesFile), "golaw_profile: 1\n")
	if err := m5KeepingCI(keeping, policy.Dirs{Config: t.TempDir()}); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("red a bundle that leaves nothing to the platform = %v", err)
	}
}

// TestM5BirthJudgesRefuseAManagedGithubPathOrNoCIFile: a policy that
// manages a .github/ path although nothing was written there, and a core
// only init of the catch-all birth that writes no CI file, the near miss
// that proves the exclusion is the profile's doing.
func TestM5BirthJudgesRefuseAManagedGithubPathOrNoCIFile(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	checkedWrite(t, filepath.Join(repo, m5GithubDir, m5CodeOwners), m5CatchAll)
	checkedWrite(t, filepath.Join(repo, policyFile), "golaw: 1\nmanaged:\n  - { path: .github/workflows/ci.yml, sha256: "+strings.Repeat("c", 64)+" }\n")
	if err := m5NoGithubWrite(repo); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("red a managed .github/ path = %v", err)
	}
	s := m2TestSuite(t).m5Scope(m5AdoptionCategory, "core-birth")
	s.execute = m5Script(t, s, m5Reply{prefix: []string{commandInit, flagApply}, answer: m5Answer(result{})}).Execute
	if err := s.m5CoreBirth(t.Context()); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("red a core only init without the CI file = %v", err)
	}
}
