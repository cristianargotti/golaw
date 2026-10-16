package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks"
	"github.com/cristianargotti/golaw/internal/hook/agents"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/proof"
	"github.com/cristianargotti/golaw/internal/source"
)

// agentUnderTest is the first agent of this version.
func agentUnderTest() agents.Agent { return agents.All()[0] }

// hookBindingFor builds the hook binding of the test binding.
func hookBindingFor(t *testing.T, b binding) hookBinding {
	t.Helper()
	dirs, err := b.dirs()
	if err != nil {
		t.Fatal(err)
	}
	state, err := b.installState()
	if err != nil {
		t.Fatal(err)
	}
	return hookBinding{binding: b, dirs: dirs, state: state, now: now(), agent: agentUnderTest().ID, hookDigest: "definition"}
}

func TestHookEnrollAllowsAPositivelyUnenrolledLocation(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	h := hookBindingFor(t, b)
	outside := t.TempDir()
	repo, err := h.enroll(t.Context(), outside, outside)
	if err != nil || repo.Enrollment.State != model.EnrollmentNone {
		t.Fatalf("outside git = %+v, %v", repo, err)
	}
	root := repository(t, true)
	repo, err = h.enroll(t.Context(), root, root)
	if err != nil || repo.Enrollment.State != model.EnrollmentNone || repo.Root != root {
		t.Fatalf("unenrolled repository = %+v, %v", repo, err)
	}
}

func TestHookEnrollPolicyAndSnapshotOfAnEnrolledRepository(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	h := hookBindingFor(t, b)
	root := repository(t, false)
	data := renderedPolicy(t)
	p, err := b.compile(data, now())
	if err != nil {
		t.Fatal(err)
	}
	if err = b.enroll(root, data, p, now()); err != nil {
		t.Fatal(err)
	}
	repo, err := h.enroll(t.Context(), root, root)
	if err != nil || repo.Enrollment.State != model.EnrollmentEnrolled || repo.Enrollment.PolicyDigest != p.Digest || repo.Identity == "" {
		t.Fatalf("enrolled repository = %+v, %v", repo, err)
	}
	_, identity, err := install.ProjectIdentity(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(agents.ObservationPath(h.state, h.agent, identity)); statErr != nil {
		t.Fatalf("no observation was recorded: %v", statErr)
	}
	loaded, err := h.policy(t.Context(), repo)
	if err != nil || loaded.Enrollment.State != model.EnrollmentEnrolled {
		t.Fatalf("policy = %+v, %v", loaded, err)
	}
	snapshot, err := h.snapshot(t.Context(), repo)
	if err != nil || snapshot.Class("golaw.yml") == model.ClassUnknown {
		t.Fatalf("snapshot = %v, %v", snapshot, err)
	}
	if err = os.RemoveAll(filepath.Join(root, ".git")); err != nil {
		t.Fatal(err)
	}
	if _, err = h.enroll(t.Context(), root, root); !errors.Is(err, model.ErrEnrollment) {
		t.Fatalf("an enrolled repository without git was not a denial: %v", err)
	}
}

func TestHookPolicyDeniesADeletedOrWeakenedProposal(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	h := hookBindingFor(t, b)
	root := repository(t, false)
	data := renderedPolicy(t)
	p, err := b.compile(data, now())
	if err != nil {
		t.Fatal(err)
	}
	if err = b.enroll(root, data, p, now()); err != nil {
		t.Fatal(err)
	}
	repo, err := h.enroll(t.Context(), root, root)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "golaw.yml")
	weakened := bytes.Replace(data, []byte("file_lines: 300"), []byte("file_lines: 301"), 1)
	if err = os.WriteFile(file, weakened, 0o600); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", "golaw.yml")
	if _, err = h.policy(t.Context(), repo); !errors.Is(err, model.ErrPolicy) {
		t.Fatalf("a staged weakening was accepted: %v", err)
	}
	if err = os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", "golaw.yml")
	if loaded, policyErr := h.policy(t.Context(), repo); policyErr != nil || loaded.Enrollment.State != model.EnrollmentEnrolled {
		t.Fatalf("the policy in force was refused: %+v, %v", loaded, policyErr)
	}
	if err = os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if _, err = h.policy(t.Context(), repo); !errors.Is(err, model.ErrEnrollment) {
		t.Fatalf("a deleted policy file was accepted: %v", err)
	}
}

func TestHookPolicyDeniesAStagedDeletionOfThePolicyAtHead(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	h := hookBindingFor(t, b)
	root := repository(t, true)
	data := renderedPolicy(t)
	p, err := b.compile(data, now())
	if err != nil {
		t.Fatal(err)
	}
	if err = b.enroll(root, data, p, now()); err != nil {
		t.Fatal(err)
	}
	repo, err := h.enroll(t.Context(), root, root)
	if err != nil {
		t.Fatal(err)
	}
	if loaded, policyErr := h.policy(t.Context(), repo); policyErr != nil || loaded.Enrollment.State != model.EnrollmentEnrolled {
		t.Fatalf("the committed policy was refused: %+v, %v", loaded, policyErr)
	}
	gitRun(t, root, "rm", "-q", "--cached", "golaw.yml")
	if _, err = os.Stat(filepath.Join(root, "golaw.yml")); err != nil {
		t.Fatalf("the checkout lost the file: %v", err)
	}
	if _, err = h.policy(t.Context(), repo); !errors.Is(err, model.ErrEnrollment) || !strings.Contains(err.Error(), "deletion of golaw.yml is staged") {
		t.Fatalf("a staged deletion was accepted: %v", err)
	}
	unreadable := &policy.Loaded{Origin: policy.OriginHead, Root: filepath.Join(root, "absent")}
	if err = stagedDeletion(t.Context(), unreadable); err == nil || errors.Is(err, model.ErrEnrollment) {
		t.Fatalf("an unreadable index = %v", err)
	}
}

func TestHookOverlayFoldsTheReconstructedCandidates(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	h := hookBindingFor(t, b)
	base := proof.NewTree(map[string][]byte{"docs/a.md": []byte("one\n")})
	edited := model.Candidate{Path: "docs/a.md", Op: model.CandidateEdit, Reconstructed: true, BaseHash: source.ContentHash([]byte("one\n"))}
	created := model.Candidate{Path: "docs/b.md", Op: model.CandidateCreate, Reconstructed: true}
	content := map[string][]byte{"docs/a.md": []byte("two\n"), "docs/b.md": []byte("new\n")}
	overlay, err := h.overlay(t.Context(), base, []model.Candidate{edited, created}, content)
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range content {
		if got, err := overlay.Read(path); err != nil || !bytes.Equal(got, want) {
			t.Fatalf("%s = %q, %v", path, got, err)
		}
	}
	stale := edited
	stale.BaseHash = source.ContentHash([]byte("other\n"))
	if _, err := h.overlay(t.Context(), base, []model.Candidate{stale}, content); !errors.Is(err, source.ErrBaseMismatch) {
		t.Fatalf("a stale base was folded: %v", err)
	}
}

func TestHookDependenciesBindTheCatalogAndTheState(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	deps, err := b.hookDependencies(agentUnderTest(), now())
	if err != nil {
		t.Fatal(err)
	}
	b.env = func(string) string { return "" }
	if _, homeless := b.hookDependencies(agentUnderTest(), now()); homeless == nil {
		t.Fatal("the hook dependencies were built without a state directory")
	}
	if deps.Registry == nil || deps.Registry.Len() != len(checks.Catalog()) {
		t.Fatalf("registry = %v", deps.Registry)
	}
	if !strings.HasSuffix(deps.State.Dir, hookStateDir) || deps.State.Now != now() || deps.Budget != 0 {
		t.Fatalf("state = %+v, budget = %v", deps.State, deps.Budget)
	}
	if deps.Enroll == nil || deps.Policy == nil || deps.Snapshot == nil || deps.Overlay == nil {
		t.Fatal("a callback is nil")
	}
}
