package goldentest

import (
	"context"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/checks"
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/lint"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// noSurface is why the surface step proves nothing over a slice.
const noSurface = "no file the commands wrote is a surface file"

// scan runs the kit's catalog over the filled module as golaw scan does
// and fails each enforced finding on a file the commands wrote or on no
// file; the base is the fixture's contracts, judged by their own tests.
func (h *harness) scan(ctx context.Context) {
	snap, err := snapshot(ctx, h.repo.Root)
	if err != nil {
		h.failf(string(StepScan), "%v", err)
		return
	}
	registry, err := checks.Registry()
	if err != nil {
		h.failf(string(StepScan), "%v", err)
		return
	}
	run, err := engine.Dispatch(ctx, registry, model.Input{Mode: model.ModeScan, Policy: h.policy, Snapshot: snap, Targets: snap.Files()})
	if err != nil {
		h.failf(string(StepScan), "%v", err)
		return
	}
	decision := engine.Decide(run)
	for i := range decision.Findings {
		f := &decision.Findings[i]
		if _, written := h.report.Files[f.Path]; f.Enforced() && (written || f.Path == "") {
			h.failf(string(StepScan), "%s:%d %s %s", f.Path, f.Line, f.Check, f.Message)
		}
	}
	for _, left := range decision.Unmeasured {
		h.failf(string(StepScan), "unmeasured %s: %s", left.Check, left.Reason)
	}
}

// surfaceLanguages answers the languages of the non-Go files the commands
// wrote, sorted.
func (h *harness) surfaceLanguages() []string {
	found := map[string]bool{}
	for path, content := range h.report.Files {
		if strings.HasSuffix(path, goSuffix) {
			continue
		}
		if info, err := surface.Detect(path, content); err == nil && info.Language != "" {
			found[string(info.Language)] = true
		}
	}
	return slices.Sorted(maps.Keys(found))
}

// surface lints the non-Go files the commands wrote with the pinned
// surface tools found on PATH; a tool that is absent skips the step.
func (h *harness) surface(ctx context.Context) {
	languages := h.surfaceLanguages()
	if len(languages) == 0 {
		h.skip(StepSurface, noSurface)
		return
	}
	snap, err := snapshot(ctx, h.repo.Root)
	if err != nil {
		h.failf(string(StepSurface), "%v", err)
		return
	}
	scope, err := surface.ScopeOf(ctx, snap, h.policy)
	if err != nil {
		h.failf(string(StepSurface), "%v", err)
		return
	}
	// The set is the strict one init writes into a new policy, the set an
	// adopter's make lint-surface runs, whatever the slice's golaw.yml says.
	set := h.surfaceSet
	if set == "" {
		set = model.SurfaceStrict
	}
	run, err := lint.Run(ctx, lint.Options{
		Root: h.repo.Root, Snapshot: snap, Policy: h.policy, Scope: scope, Only: languages, Set: set,
		Resolver: toolchain.Resolver{
			Dir: filepath.Join(h.work, "tools"), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
			Env: []string{"PATH=" + toolPath(languages)}, Run: runner.Run,
		},
		Kit: h.policy.KitVersion,
	})
	if err != nil {
		h.failf(string(StepSurface), "%v", err)
		return
	}
	h.judgeSurface(run)
}

// judgeSurface skips with the reason of every outcome a tool could not
// measure or had no rule for, so the step never passes on what nothing
// judged, and fails each finding on a file the commands wrote.
func (h *harness) judgeSurface(run *engine.Run) {
	var reasons []string
	for i := range run.Outcomes {
		status := run.Outcomes[i].Result.Status
		if status.Kind == model.StatusUnmeasured || status.Kind == model.StatusNotApplicable {
			reasons = append(reasons, status.Reason)
		}
	}
	if len(reasons) != 0 {
		h.skip(StepSurface, strings.Join(reasons, "; "))
	}
	findings := run.Findings()
	for i := range findings {
		f := &findings[i]
		if _, written := h.report.Files[f.Path]; written {
			h.failf(string(StepSurface), "%s:%d %s %s", f.Path, f.Line, f.Check, f.Message)
		}
	}
}

// toolPath is the PATH of the surface tools of the languages: the
// directories where the tools of their drivers are found.
func toolPath(languages []string) string {
	dirs := map[string]bool{}
	for _, d := range lint.Drivers() {
		reads := slices.ContainsFunc(d.Languages, func(l surface.Language) bool { return slices.Contains(languages, string(l)) })
		if !reads {
			continue
		}
		if path, err := exec.LookPath(d.Tool); err == nil { // fallback: a tool absent from PATH leaves its outcome unmeasured, which skips the step
			dirs[filepath.Dir(path)] = true
		}
	}
	return strings.Join(slices.Sorted(maps.Keys(dirs)), string(os.PathListSeparator))
}
