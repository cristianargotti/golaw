package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cristianargotti/golaw/internal/coverage"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/newcode"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/surface"
)

// unmeasuredNewCodeError is a cause the measure could not get past: the
// command prints it as the report's reason and exits 2.
type unmeasuredNewCodeError struct{ reason string }

func (e *unmeasuredNewCodeError) Error() string { return e.reason }

// newCodeRun carries one measure from the policy in force to the report;
// header holds the lines known so far, so an unmeasured answer still names
// the base and the report it read.
type newCodeRun struct {
	root      string
	freeze    func(ctx context.Context, root string) (model.Snapshot, error)
	policy    *model.EffectivePolicy
	options   coverageOptions
	threshold int
	header    []string
}

// coverageNew measures the new code of the repository that holds root
// against the threshold of the coverage floors (M6.8). It arms nothing and
// writes nothing: it reads the pipeline report and git.
func (b binding) coverageNew(
	ctx context.Context, root string, now time.Time, options coverageOptions,
) (newcode.Report, []string, error) {
	if err := options.validateNew(); err != nil {
		return newcode.Report{}, nil, err
	}
	loaded, err := b.loaded(ctx, root, now)
	if err != nil {
		return newcode.Report{}, nil, err
	}
	if checkoutErr := enrolledCheckout(loaded); checkoutErr != nil {
		return newcode.Report{}, nil, checkoutErr
	}
	if !loaded.Policy.Has(goCapability) {
		return newcode.Report{}, nil, coverageOffError{}
	}
	run := &newCodeRun{
		root: loaded.Root, freeze: b.snapshot, policy: loaded.Policy, options: options,
		threshold: coverage.FloorsFrom(loaded.Policy.OptionsFor(policy.CoverageCheck)).NewCode,
	}
	report, err := run.measure(newcode.WithEnviron(ctx, b.environ()))
	if cause, ok := errors.AsType[*unmeasuredNewCodeError](err); ok {
		return newcode.Report{Threshold: run.threshold, Status: model.Unmeasured(cause.reason)}, run.header, nil
	}
	if err != nil {
		return newcode.Report{}, nil, err
	}
	return report, run.header, nil
}

func (r *newCodeRun) measure(ctx context.Context) (newcode.Report, error) {
	snapshot, scope, err := r.snapshot(ctx)
	if err != nil {
		return newcode.Report{}, err
	}
	base, err := r.base(ctx)
	if err != nil {
		return newcode.Report{}, err
	}
	input := r.input(scope)
	r.header = []string{fmt.Sprintf("coverage on new code: base %s, report %s, analyzer scope %s", base, input, scopeLabel(scope))}
	universe := newcode.Universe(snapshot, scope)
	hits, err := r.report(snapshot, scope, input, universe)
	if err != nil {
		return newcode.Report{}, err
	}
	added, err := newcode.NewLines(ctx, r.root, base)
	if err != nil {
		return newcode.Report{}, fmt.Errorf("the new lines since %s: %w", base, err)
	}
	return newcode.Measure(newcode.Input{Lines: hits, NewLines: added, Universe: universe, Threshold: r.threshold}), nil
}

// snapshot freezes the worktree through the binding's seam and resolves
// the analyzer scope over it; a scope that could not read a list leaves
// the universe unknown.
func (r *newCodeRun) snapshot(ctx context.Context) (model.Snapshot, surface.Scope, error) {
	snapshot, err := r.freeze(ctx, r.root)
	if err != nil {
		return nil, surface.Scope{}, err
	}
	scope, err := surface.ScopeOf(ctx, snapshot, r.policy)
	if err != nil {
		return nil, surface.Scope{}, fmt.Errorf("resolve the analyzer scope of %s: %w", r.root, err)
	}
	if len(scope.Unmeasured) > 0 {
		return nil, surface.Scope{}, &unmeasuredNewCodeError{"the analyzer scope is unmeasured: " + strings.Join(scope.Unmeasured, "; ")}
	}
	return snapshot, scope, nil
}

// base is the start of the new code period: the named ref, the last tag,
// or the remote default branch, whose merge base NewLines takes.
func (r *newCodeRun) base(ctx context.Context) (string, error) {
	switch {
	case r.options.Base != "":
		return r.options.Base, nil
	case r.options.SinceTag:
		tag, err := newcode.LastTag(ctx, r.root)
		if err != nil {
			return "", fmt.Errorf("the last tag: %w", err)
		}
		return tag, nil
	}
	branch, err := newcode.DefaultBranch(ctx, r.root)
	if err != nil {
		return "", fmt.Errorf("the remote default branch: %w", err)
	}
	if branch == "" {
		return "", fmt.Errorf("%w: the remote default branch is unknown here, name --base <ref> or --since-tag", model.ErrUsage)
	}
	return branch, nil
}

// input keeps --input relative to the invocation directory, as golaw
// coverage reads it; the scope's report lives under the repository root.
func (r *newCodeRun) input(scope surface.Scope) string {
	if r.options.Input != "" {
		return r.options.Input
	}
	return filepath.Join(r.root, filepath.FromSlash(scope.CoverageReport))
}

// report reads the lines to cover of a report written after every Go file
// of the universe; a missing or stale report names the command that
// writes it in the pipeline's shape.
func (r *newCodeRun) report(
	snapshot model.Snapshot, scope surface.Scope, input string, universe func(string) bool,
) (map[string]newcode.LineHits, error) {
	info, err := os.Stat(input)
	if err != nil {
		return nil, &unmeasuredNewCodeError{fmt.Sprintf("the report %s cannot be read: write it with %s first", input, pipelineShape(scope, input))}
	}
	stale, err := newcode.Stale(info.ModTime(), snapshot, universe, modTimeUnder(r.root))
	if err != nil {
		return nil, &unmeasuredNewCodeError{err.Error()}
	}
	if stale {
		return nil, &unmeasuredNewCodeError{fmt.Sprintf("the report %s is older than the Go files in scope: write it again with %s", input, pipelineShape(scope, input))}
	}
	return readNewCodeReport(snapshot, input)
}

// readNewCodeReport keeps the ErrReport text and the path in the reason,
// so the person sees which report and which line golaw could not read.
func readNewCodeReport(snapshot model.Snapshot, input string) (map[string]newcode.LineHits, error) {
	gomod, err := snapshot.Read(initModuleFile)
	if err != nil {
		return nil, &unmeasuredNewCodeError{"the root holds no readable go.mod: " + err.Error()}
	}
	module, err := newcode.Module(gomod)
	if err != nil {
		return nil, &unmeasuredNewCodeError{err.Error()}
	}
	file, err := os.Open(input) //nolint:gosec // the person or the analyzer scope names the report
	if err != nil {
		return nil, &unmeasuredNewCodeError{fmt.Sprintf("open the report %s: %s", input, err)}
	}
	hits, readErr := newcode.Lines(file, module, snapshot.Read)
	if err := errors.Join(readErr, file.Close()); err != nil {
		return nil, &unmeasuredNewCodeError{fmt.Sprintf("the report %s: %s", input, err)}
	}
	return hits, nil
}

// pipelineShape is the go test line that writes the report the pipeline
// passes (M6.8 item 1), so the fix a reason names writes the right report.
func pipelineShape(scope surface.Scope, input string) string {
	parts := []string{"go test"}
	if scope.Coverage.CoverPkg {
		parts = append(parts, "-coverpkg=./...")
	}
	if mode := scope.Coverage.CoverMode; mode != "" && mode != "set" {
		parts = append(parts, "-covermode="+mode)
	}
	return strings.Join(append(parts, "-coverprofile", input, "./..."), " ")
}

func scopeLabel(scope surface.Scope) string {
	if scope.Origin == "" {
		return scope.Source
	}
	return scope.Source + " (" + scope.Origin + ")"
}

func modTimeUnder(root string) func(string) (time.Time, error) {
	return func(rel string) (time.Time, error) {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return time.Time{}, fmt.Errorf("stat %s: %w", rel, err)
		}
		return info.ModTime(), nil
	}
}
