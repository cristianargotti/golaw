package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gen/apply"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gen/thirdparty"
	"github.com/cristianargotti/golaw/internal/genheader"
	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/source"
)

// genLockName is the lock of the repository's generation, tried by the hook
// without waiting, so a second event never blocks the agent on the first.
const genLockName = "gen"

// regenWork is what a post event asks of the regeneration: the contracts
// it wrote, the regenerable files it edited by hand, the Go files the
// mechanical fixes apply to and the pinned generators its inputs feed.
type regenWork struct {
	contracts, edited, fix []string
	third                  []thirdparty.Generator
}

func (w regenWork) empty() bool {
	return len(w.contracts) == 0 && len(w.edited) == 0 && len(w.fix) == 0 && len(w.third) == 0
}

// regenOutcome is everything the lines of a regeneration are written from.
type regenOutcome struct {
	contracts []string
	labels    map[string]string
	written   int
	skipped   []apply.Skip
	result    gen.Result
	drift     []gen.Drift
	// driftContracts names the contract of each drifted regenerable file,
	// what the agent edits instead (M7.9.2).
	driftContracts map[string]string
	fixed          []fixedFile
	causes         []string
	outside        []string
}

// regenerate is the post hook's regeneration (M7.9.1): it never denies, and
// what it cannot do in the hook becomes a notice naming golaw gen. It reads
// the snapshot of the post event, which the cache lists once after the tool
// wrote and reads again from disk.
func (h hookBinding) regenerate(ctx context.Context, repository hook.Repository, pol *model.EffectivePolicy, written []string) hook.RegenReport {
	language := pol.Language
	if language == "" {
		language = model.DefaultLanguage
	}
	if !worthRegen(repository.Root, written) {
		return hook.RegenReport{}
	}
	snap, err := h.snapshot(ctx, repository)
	if err != nil {
		return regenLines(language, regenOutcome{causes: []string{err.Error()}})
	}
	m, err := readManifest(snap)
	if err != nil {
		return regenLines(language, regenOutcome{causes: []string{err.Error()}})
	}
	work := planRegen(snap, pol, m, written)
	if work.empty() {
		return hook.RegenReport{}
	}
	release, err := h.genLock(ctx, repository.Root)
	if errors.Is(err, runner.ErrLocked) {
		return deferredLines(language)
	}
	if err != nil {
		return regenLines(language, regenOutcome{causes: []string{err.Error()}})
	}
	defer release()
	return regenLines(language, h.regenerateLocked(ctx, repository.Root, snap, pol, m, work))
}

// planRegen sorts the written paths into the work of the event; without a
// manifest only the mechanical fixes apply, because nothing was generated,
// and no pinned generator runs over contracts the manifest never named.
func planRegen(snap model.Snapshot, pol *model.EffectivePolicy, m *manifest.Manifest, written []string) regenWork {
	var w regenWork
	if m != nil {
		w.third = thirdparty.Triggered(thirdparty.Generators(pol, snap), written)
	}
	for _, p := range written {
		regenerable := m != nil && generatedPath(&w, *m, p)
		if !regenerable && fixable(snap, p) {
			w.fix = append(w.fix, p)
		}
	}
	return w
}

// generatedPath files p under the contracts or the hand edited regenerable
// files of the manifest, and reports whether a run renders p again.
func generatedPath(w *regenWork, m manifest.Manifest, p string) bool {
	if len(m.Contracting(p)) > 0 && !slices.Contains(w.contracts, p) {
		w.contracts = append(w.contracts, p)
	}
	if _, ok := m.Regenerable(p); !ok {
		return false
	}
	if !slices.Contains(w.edited, p) {
		w.edited = append(w.edited, p)
	}
	return true
}

// fixable reports a Go file the agent owns: a file of another generator or
// of golaw carries its own formatting, and a fixture is kept as written.
func fixable(snap model.Snapshot, p string) bool {
	owned := []model.FileClass{model.ClassTracked, model.ClassUntracked, model.ClassTest}
	if !strings.HasSuffix(p, goExt) || !slices.Contains(owned, snap.Class(p)) {
		return false
	}
	data, err := snap.Read(p)
	if err != nil {
		return false
	}
	_, generated := genheader.Parse(data)
	return !generated
}

// worthRegen answers, without asking git, whether the event can need the
// regeneration: a Go file was written or the repository holds a manifest.
// Every other event pays nothing, so the git work of the post event stays
// what it was before the generators.
func worthRegen(root string, written []string) bool {
	if slices.ContainsFunc(written, func(p string) bool { return strings.HasSuffix(p, goExt) }) {
		return true
	}
	_, err := os.Lstat(source.OSPath(root, manifest.Path))
	return !errors.Is(err, fs.ErrNotExist)
}

// genLock takes the generation lock without waiting; a lock file a crash
// leaves is freed by the next acquire, so the release error is dropped.
func (h hookBinding) genLock(ctx context.Context, root string) (func(), error) {
	repo, err := h.cache.Discover(ctx, root)
	if err != nil {
		return nil, discoverFailed(root, err)
	}
	lock, err := runner.Acquire(ctx, runner.LockPath(repo.GitDir, genLockName), 0)
	if err != nil {
		return nil, fmt.Errorf("take the generation lock: %w", err)
	}
	return func() { _ = lock.Release() }, nil //nolint:errcheck // a stale lock is freed by the next acquire
}

// regenerateLocked renders the changed contracts, reads back the drift of
// the hand edited regenerable files and fixes the Go files the event wrote.
func (h hookBinding) regenerateLocked(ctx context.Context, root string, snap model.Snapshot, pol *model.EffectivePolicy, m *manifest.Manifest, work regenWork) regenOutcome {
	o := regenOutcome{contracts: work.contracts}
	snap = h.regenThird(ctx, root, snap, work.third, &o)
	var appended []string
	if m != nil && len(work.contracts) > 0 {
		o.labels = contractLabels(*m, work.contracts)
		appended = regenContracts(ctx, root, snap, pol, *m, &o)
	}
	if m != nil && len(work.edited) > 0 {
		editedDrift(ctx, snap, pol, *m, work.edited, &o)
	}
	module, err := moduleOf(snap)
	if err != nil {
		module = ""
	}
	event, cancel := h.budget.context(ctx)
	defer cancel()
	o.fixed = fixFiles(event, root, module, snap, append(slices.Clone(work.fix), appended...), h.lintFmt(root))
	return o
}

// regenContracts regenerates the entries of the changed contracts and lands
// the regenerable files and the appended stubs; it answers the paths of the
// stubs, which the mechanical fixes then read.
func regenContracts(ctx context.Context, root string, snap model.Snapshot, pol *model.EffectivePolicy, m manifest.Manifest, o *regenOutcome) []string {
	in, err := hookGenInput(snap, pol, m, o.contracts)
	if err != nil {
		o.causes = append(o.causes, err.Error())
		return nil
	}
	res, err := gen.Generate(ctx, in)
	if err != nil {
		o.causes = append(o.causes, err.Error())
		return nil
	}
	o.result = res
	writes := hookWrites(res.Writes)
	report, err := apply.Files(root, writes)
	o.written, o.skipped = len(report.Written), report.Skipped
	if err != nil {
		o.causes = append(o.causes, err.Error())
	}
	var appended []string
	for _, w := range writes {
		if w.Mode == gen.WriteAppend && slices.Contains(report.Written, w.Path) {
			appended = append(appended, w.Path)
		}
	}
	return appended
}

// hookWrites keeps the regenerable files and the appended stubs: the hook
// never writes an owned file whole, golaw.yml or the manifest, which only
// golaw new and golaw gen write.
func hookWrites(writes []gen.Write) []gen.Write {
	var out []gen.Write
	for _, w := range writes {
		if w.Mode == gen.WriteRegenerable || w.Mode == gen.WriteAppend {
			out = append(out, w)
		}
	}
	return out
}

// editedDrift reports a hand edit of a regenerable file as drift and leaves
// the bytes as the agent wrote them (A46): the fix is the contract, which
// golaw gen then renders.
func editedDrift(ctx context.Context, snap model.Snapshot, pol *model.EffectivePolicy, m manifest.Manifest, edited []string, o *regenOutcome) {
	var changed []string
	for _, p := range edited {
		e, _ := m.Regenerable(p)
		if len(e.Contracts) == 0 {
			changed = nil
			break
		}
		for _, c := range e.Contracts {
			changed = append(changed, c.Path)
		}
	}
	in, err := hookGenInput(snap, pol, m, changed)
	if err != nil {
		o.causes = append(o.causes, err.Error())
		return
	}
	res, err := gen.Check(ctx, in)
	if err != nil {
		o.causes = append(o.causes, err.Error())
		return
	}
	for _, d := range res.Drift {
		if slices.Contains(edited, d.Path) {
			o.addDrift(m, d)
		}
	}
}

// addDrift records one drift with the contract its manifest entry names.
func (o *regenOutcome) addDrift(m manifest.Manifest, d gen.Drift) {
	if e, ok := m.Regenerable(d.Path); ok && len(e.Contracts) > 0 {
		if o.driftContracts == nil {
			o.driftContracts = map[string]string{}
		}
		o.driftContracts[d.Path] = e.Contracts[0].Path
	}
	o.drift = append(o.drift, d)
}
