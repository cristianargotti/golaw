package main

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/lint"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// lint loads the policy as coverage does, so CI lints a checkout without
// an enrollment record, and runs the pinned linters over the analyzer
// scope of the worktree.
func (b binding) lint(ctx context.Context, path string, now time.Time, opts lintOptions) (model.Input, *engine.Run, error) {
	loaded, err := b.loaded(ctx, path, now)
	if err != nil {
		return model.Input{}, nil, err
	}
	if checkoutErr := enrolledCheckout(loaded); checkoutErr != nil {
		return model.Input{}, nil, checkoutErr
	}
	snapshot, err := b.snapshot(ctx, loaded.Root)
	if err != nil {
		return model.Input{}, nil, err
	}
	scope, err := surface.ScopeOf(ctx, snapshot, loaded.Policy)
	if err != nil {
		return model.Input{}, nil, fmt.Errorf("resolve the analyzer scope of %s: %w", loaded.Root, err)
	}
	// A machine with no tools directory resolves every tool missing, so the
	// run is Unmeasured and names golaw setup --surface instead of failing.
	dirs, dirsErr := b.dirs()
	if dirsErr != nil {
		dirs = policy.Dirs{}
	}
	run, err := lint.Run(ctx, lint.Options{
		Root: loaded.Root, Snapshot: snapshot, Policy: loaded.Policy, Scope: scope,
		Set: opts.set, Only: opts.only, Resolver: b.lintResolver(dirs), Kit: b.pin(),
	})
	if err != nil {
		return model.Input{}, nil, fmt.Errorf("lint %s: %w", loaded.Root, err)
	}
	return model.Input{Mode: model.ModeScan, Policy: loaded.Policy, Snapshot: snapshot}, run, nil
}

// lintResolver finds the pinned tools in the tool directory and on the
// PATH of the command's environment.
func (b binding) lintResolver(dirs policy.Dirs) toolchain.Resolver {
	return toolchain.Resolver{Dir: dirs.Tools, GOOS: b.goos, GOARCH: runtime.GOARCH, Env: b.environ(), Run: runner.Run}
}
