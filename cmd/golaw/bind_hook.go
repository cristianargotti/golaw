package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/checks"
	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/agents"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/source"
)

// hookStateDir is the directory under the state directory that holds the
// bounded hook state of A15.
const hookStateDir = "hook"

// hookBinding binds the callbacks of hook.Dependencies for one hook
// process: the enrollment, the policy in force, the worktree snapshot and
// the candidate overlay, all from the source and policy packages.
type hookBinding struct {
	binding

	dirs       policy.Dirs
	state      string
	now        time.Time
	agent      string
	hookDigest string
}

// hookDependencies is the hookDeps seam.
func (b binding) hookDependencies(agent agents.Agent, now time.Time) (hook.Dependencies, error) {
	dirs, err := b.dirs()
	if err != nil {
		return hook.Dependencies{}, err
	}
	state, err := b.installState()
	if err != nil {
		return hook.Dependencies{}, err
	}
	digest, err := hookDigest(agent)
	if err != nil {
		return hook.Dependencies{}, err
	}
	registry, err := checks.Registry()
	if err != nil {
		return hook.Dependencies{}, fmt.Errorf("build the registry: %w", err)
	}
	h := hookBinding{binding: b, dirs: dirs, state: state, now: now, agent: agent.ID, hookDigest: digest}
	return hook.Dependencies{
		Enroll: h.enroll, Policy: h.policy, Snapshot: h.snapshot, Overlay: h.overlay,
		Registry: registry, State: hook.Store{Dir: filepath.Join(dirs.State, hookStateDir), Now: now},
	}, nil
}

// hookDigest is the digest of the hook definition the plugin of an agent
// ships, the one doctor binds an observation to.
func hookDigest(agent agents.Agent) (string, error) {
	data, err := assets.Read("agents/" + agent.ID + "/hooks/hooks.json")
	if err != nil {
		return "", fmt.Errorf("read the hook definition: %w", err)
	}
	return install.Digest(data), nil
}

// enroll consults the enrollment first (A3): a location outside git or an
// unenrolled repository is a positively unenrolled location the hook
// allows, an enrolled repository carries its record.
func (h hookBinding) enroll(ctx context.Context, cwd string) (hook.Repository, error) {
	var found source.Repo
	discover := func(ctx context.Context, dir string) (policy.Discovery, error) {
		repo, err := source.Discover(ctx, dir)
		if err != nil {
			return policy.Discovery{}, discoverFailed(dir, err)
		}
		found = repo
		return policy.Discovery{Root: repo.Root, Head: repo.Head}, nil
	}
	loc, err := policy.Locate(ctx, h.dirs.State, cwd, discover)
	switch {
	case err == nil:
	case errors.Is(err, model.ErrEnrollment):
		return hook.Repository{}, fmt.Errorf("locate %s: %w", cwd, err)
	case errors.Is(err, source.ErrNotRepository), errors.Is(err, source.ErrGit):
		return hook.Repository{Enrollment: model.Enrollment{State: model.EnrollmentNone}}, nil
	default:
		return hook.Repository{}, fmt.Errorf("locate %s: %w", cwd, err)
	}
	repo := hook.Repository{
		Root: loc.Root, Identity: found.Identity(), CaseInsensitive: found.IgnoreCase,
		Enrollment: model.Enrollment{State: model.EnrollmentNone},
	}
	if loc.Record != nil {
		repo.Enrollment = model.Enrollment{
			State: model.EnrollmentEnrolled, PolicyDigest: loc.Record.PolicyDigest, ProfileDigest: loc.Record.ProfileDigest,
		}
		h.observe(loc.Root, cwd)
	}
	return repo, nil
}

// observe records the environment the agent gave this process, for doctor
// to resolve the binary the way the agent does. It is evidence, never a
// reason to deny, so a failed write is dropped.
func (h hookBinding) observe(root, cwd string) {
	_, identity, err := install.ProjectIdentity(root)
	if err != nil {
		return
	}
	observation := doctor.Observation{
		Identity: identity, HookDigest: h.hookDigest,
		Environment: doctor.ObserveEnvironment(h.environ(), cwd),
	}
	_ = doctor.SaveObservation(agents.ObservationPath(h.state, h.agent, identity), observation) //nolint:errcheck // evidence for doctor, never a verdict
}

// policy is the policy in force of the repository with its enrollment
// state, after the proposal in the checkout is judged (A2, A3): on an
// enrolled repository a policy file gone from the checkout and a staged
// policy that weakens the one in force are both denials naming the cause.
func (h hookBinding) policy(ctx context.Context, repo hook.Repository) (*model.EffectivePolicy, error) {
	l, err := h.loaded(ctx, repo.Root, h.now)
	if err != nil {
		return nil, err
	}
	if l.Record == nil {
		return l.Policy, nil
	}
	if _, err := os.Lstat(filepath.Join(repo.Root, policy.FileName)); err != nil {
		return nil, fmt.Errorf("%w: %s is gone from the checkout of %s (%s)", model.ErrEnrollment, policy.FileName, repo.Root, model.EnrollmentMissing)
	}
	if _, _, err := policy.Proposed(ctx, l, schemas()); err != nil {
		return nil, fmt.Errorf("staged %s: %w", policy.FileName, err)
	}
	return l.Policy, nil
}

// snapshot is the frozen worktree of the repository.
func (h hookBinding) snapshot(ctx context.Context, repo hook.Repository) (model.Snapshot, error) {
	return h.binding.snapshot(ctx, repo.Root)
}

// overlay folds the candidates of a write event, with the bytes the adapter
// reconstructed, into the base snapshot.
func (h hookBinding) overlay(_ context.Context, base model.Snapshot, candidates []model.Candidate, content map[string][]byte) (model.Snapshot, error) {
	o, _, err := source.NewOverlay(base, overlayMutations(candidates, content))
	if err != nil {
		return nil, fmt.Errorf("fold the candidates: %w", err)
	}
	return o, nil
}

// overlayMutations maps the candidates to the mutations of the source
// package. An edit the adapter already applied in memory arrives as the
// whole content, so it is folded as a replace bound to the same base.
func overlayMutations(candidates []model.Candidate, content map[string][]byte) []source.Mutation {
	muts := make([]source.Mutation, 0, len(candidates))
	for _, c := range candidates {
		m := source.Mutation{Path: c.Path, Op: c.Op, From: c.From, BaseHash: c.BaseHash, Content: content[c.Path]}
		if c.Op == model.CandidateEdit && c.Reconstructed {
			m.Op = model.CandidateReplace
		}
		muts = append(muts, m)
	}
	return muts
}
