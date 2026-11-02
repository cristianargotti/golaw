package main

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// setupGen builds the tools of genTools from the repository's tools module
// into the per-OS tool directory, a receipt per binary. The build runs the
// person's go with the person's environment, the only place it reaches the
// network, and only because a person runs setup (A29).
func (b binding) setupGen(ctx context.Context, out io.Writer, path string, now time.Time, run func(context.Context, runner.Command) (runner.Output, error)) error {
	root, err := b.discover(ctx, path)
	if err != nil {
		return err
	}
	dirs, err := b.dirs()
	if err != nil || dirs.Tools == "" {
		return fmt.Errorf("%w: no tools directory resolves on this machine: set XDG_DATA_HOME or HOME", model.ErrUsage)
	}
	pol, err := b.load(ctx, root, now)
	if err != nil {
		return err
	}
	_, err = toolchain.BuildGo(ctx, toolchain.GoBuild{
		Dir: dirs.Tools, Root: root, GOOS: b.goos, GOARCH: runtime.GOARCH,
		Tools: genTools(pol), Run: run, Env: b.environ(), Out: out,
	})
	if err != nil {
		return fmt.Errorf("build the generators: %w", err)
	}
	return nil
}
