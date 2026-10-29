package main

import (
	"context"
	"errors"
	"fmt"
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
// the candidate overlay, all from the source and policy packages. The
// cache keeps what git answered in this process, so the process asks each
// repository fact once and lists each worktree once.
type hookBinding struct {
	binding

	dirs       policy.Dirs
	state      string
	now        time.Time
	agent      string
	hookDigest string
	cache      *source.Cache
}

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
	h := hookBinding{binding: b, dirs: dirs, state: state, now: now, agent: agent.ID, hookDigest: digest, cache: source.NewCache()}
	return hook.Dependencies{
		Home: b.processHome(), Begin: h.cache.Reset,
		Enroll: h.enroll, Policy: h.policy, Snapshot: h.snapshot, Changes: h.changes, Moved: h.moved, Hashes: h.hashes, Overlay: h.overlay,
		Commits: h.commits, Committer: h.committer, Automatic: h.automatic, Remerged: h.remerged,
		AutomaticWith: h.automaticWith, RemergedWith: h.remergedWith, Conflicts: h.conflicts, Restyled: h.restyled, Parents: h.parents,
		Registry: registry, Tools: h.tools, State: hook.Store{Dir: filepath.Join(dirs.State, hookStateDir), Now: now}, Capture: h.capture,
	}, nil
}

// processHome is the home directory of the hook process, the one the host
// that runs it expands a tool's path from; it is empty when the
// environment names none, and the hook then refuses such a path.
func (b binding) processHome() string {
	home, err := b.home()
	if err != nil {
		return ""
	}
	return home
}

// hookDigest is the digest of the hook definition an agent ships, the one
// doctor binds an observation to.
func hookDigest(agent agents.Agent) (string, error) {
	data, err := assets.Read(agents.HookAsset(agent.ID))
	if err != nil {
		return "", fmt.Errorf("read the hook definition: %w", err)
	}
	return install.Digest(data), nil
}

// enroll consults the enrollment first (A3): a location outside git or an
// unenrolled repository is a positively unenrolled location the hook
// allows, an enrolled repository carries its record. dir is where the
// event's target lives; cwd is where the agent runs the hook, which the
// observation keeps for doctor.
func (h hookBinding) enroll(ctx context.Context, dir, cwd string) (hook.Repository, error) {
	var found source.Repo
	discover := func(ctx context.Context, dir string) (policy.Discovery, error) {
		repo, err := h.cache.Discover(ctx, dir)
		if err != nil {
			return policy.Discovery{}, discoverFailed(dir, err)
		}
		found = repo
		return policy.Discovery{Root: repo.Root, Head: repo.Head}, nil
	}
	loc, err := policy.Locate(ctx, h.dirs.State, dir, discover)
	switch {
	case err == nil:
	case errors.Is(err, model.ErrEnrollment):
		return hook.Repository{}, fmt.Errorf("locate %s: %w", dir, err)
	case errors.Is(err, source.ErrNotRepository), errors.Is(err, source.ErrGit):
		return hook.Repository{Enrollment: model.Enrollment{State: model.EnrollmentNone}}, nil
	default:
		return hook.Repository{}, fmt.Errorf("locate %s: %w", dir, err)
	}
	repo := hook.Repository{
		Root: loc.Root, Identity: found.Identity(), CaseInsensitive: found.IgnoreCase,
		Enrollment: model.Enrollment{State: model.EnrollmentNone}, Head: found.Head,
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
		SeenAt:      h.now, OS: h.goos,
	}
	_ = doctor.SaveObservation(agents.ObservationPath(h.state, h.agent, identity), observation) //nolint:errcheck // evidence for doctor, never a verdict
}

// policy is the policy in force of the repository with its enrollment
// state, after the proposal in the checkout is judged (A2, A3): on an
// enrolled repository a policy file gone from the checkout, a staged
// deletion of the file HEAD holds and a staged policy that weakens the one
// in force are all denials naming the cause. While HEAD holds no golaw.yml
// yet, the enrolled policy of the index or the checkout is in force, and
// any other state asks for the commit.
func (h hookBinding) policy(ctx context.Context, repo hook.Repository) (*model.EffectivePolicy, error) {
	l, err := h.loadedBy(ctx, policy.Input{Dir: repo.Root, Now: h.now, Discover: h.locate, Uncommitted: true})
	if err != nil {
		return nil, err
	}
	if l.Record == nil {
		return l.Policy, nil
	}
	if gone := enrolledCheckout(l); gone != nil {
		return nil, gone
	}
	proposal, err := policy.Propose(ctx, l, schemas())
	if err != nil {
		return nil, fmt.Errorf("staged %s: %w", policy.FileName, err)
	}
	if deleted := stagedDeletion(l, proposal.Staged); deleted != nil {
		return nil, deleted
	}
	return l.Policy, nil
}

// stagedDeletion refuses an index without golaw.yml while HEAD holds it:
// Proposed reads such an index as nothing staged, yet the next commit
// deletes the policy in force. On an unborn branch the bootstrap record
// governs and the untracked file init --apply wrote is the expected state,
// so only a policy in force from HEAD is checked.
func stagedDeletion(l *policy.Loaded, staged bool) error {
	if l.Origin != policy.OriginHead || staged {
		return nil
	}
	return fmt.Errorf("%w: the deletion of %s is staged in %s (%s)", model.ErrEnrollment, policy.FileName, l.Root, model.EnrollmentMissing)
}

// locate is the discovery the policy load uses, through the cache, so the
// load asks git nothing the enrollment already asked.
func (h hookBinding) locate(ctx context.Context, dir string) (policy.Discovery, error) {
	repo, err := h.cache.Discover(ctx, dir)
	if err != nil {
		return policy.Discovery{}, discoverFailed(dir, err)
	}
	return policy.Discovery{Root: repo.Root, Head: repo.Head}, nil
}

// snapshot is the worktree of the repository through the cache: listed
// once, and read again from disk by every later snapshot of the process.
func (h hookBinding) snapshot(ctx context.Context, repo hook.Repository) (model.Snapshot, error) {
	found, err := h.cache.Discover(ctx, repo.Root)
	if err != nil {
		return nil, discoverFailed(repo.Root, err)
	}
	w, err := h.cache.Worktree(ctx, found)
	if err != nil {
		return nil, fmt.Errorf("freeze the worktree of %s: %w", found.Root, err)
	}
	return w, nil
}

// changes is what git status reports changed in the repository, against
// HEAD, the index or as untracked, as a snapshot of those paths and of the
// paths of also only, so the content identity of a shell command hashes
// nothing else, with the HEAD it compared with, the blobs HEAD holds at
// those paths, what a merge staged, with the blobs of the index, or left
// unmerged and the merge in progress. It is asked once per event and never
// kept.
func (h hookBinding) changes(ctx context.Context, repo hook.Repository, also []string) (hook.Changes, error) {
	found, err := h.cache.Discover(ctx, repo.Root)
	if err != nil {
		return hook.Changes{}, discoverFailed(repo.Root, err)
	}
	st, err := source.Changes(ctx, found, also)
	if err != nil {
		return hook.Changes{}, fmt.Errorf("list the changes of %s: %w", found.Root, err)
	}
	return hook.Changes{
		Snapshot: st.Worktree, Paths: st.Paths, Head: found.Head, Blobs: st.Heads, Edited: st.Edited,
		Staged: st.Staged, Index: st.Index, Unmerged: st.Unmerged, Merge: st.Merge,
	}, nil
}

// moved is what differs between two commits, with the blob the earlier one
// held at each path, in one question to git.
func (h hookBinding) moved(ctx context.Context, repo hook.Repository, from, to string) (map[string]string, error) {
	found, err := h.cache.Discover(ctx, repo.Root)
	if err != nil {
		return nil, discoverFailed(repo.Root, err)
	}
	moved, err := source.Moved(ctx, found, from, to)
	if err != nil {
		return nil, fmt.Errorf("read the move of %s from %s: %w", found.Root, from, err)
	}
	return moved, nil
}

// hashes is the blob git would store for the bytes of each path now,
// through the conversions of the path, in one question to git.
func (h hookBinding) hashes(ctx context.Context, repo hook.Repository, paths []string) (map[string]string, error) {
	found, err := h.cache.Discover(ctx, repo.Root)
	if err != nil {
		return nil, discoverFailed(repo.Root, err)
	}
	hashes, err := source.HashObjects(ctx, found, paths)
	if err != nil {
		return nil, fmt.Errorf("name the bytes of %s as git stores them: %w", found.Root, err)
	}
	return hashes, nil
}

// overlay folds the candidates of a write event, with the bytes the adapter
// reconstructed, into the base snapshot.
func (h hookBinding) overlay(_ context.Context, base model.Snapshot, candidates []model.Candidate, content map[string][]byte) (model.Snapshot, error) {
	o, _, err := source.NewOverlay(base, source.FromCandidates(candidates, content))
	if err != nil {
		return nil, fmt.Errorf("fold the candidates: %w", err)
	}
	return o, nil
}
