package toolchain

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/cristianargotti/golaw/internal/runner"
)

// The hook budgets of A28. A tool killed at its budget is not measured and
// the pipeline judges it; the event stops at three seconds so the internal
// budget of the hook (8 s) and the host's 10 s are never reached.
const (
	HookToolBudget  = 1500 * time.Millisecond
	HookEventBudget = 3 * time.Second
)

// Request is one run of a tool.
type Request struct {
	Path    string
	Args    []string
	Stdin   []byte // the candidate bytes, never a temporary file (M6.6.1)
	Dir     string
	Env     []string
	Timeout time.Duration // zero runs under ctx alone
	// Clock measures Elapsed; nil leaves Elapsed at zero. cmd passes
	// time.Now, because no package below cmd reads the clock.
	Clock func() time.Time
}

// Execution is what a run answered.
type Execution struct {
	Output  runner.Output
	Killed  bool // the tool's own budget ended it, so it judged nothing
	Elapsed time.Duration
}

// Exec runs one tool under its budget. A tool killed at its budget is an
// Execution with Killed and no error, so the event goes on with the other
// tools; an event the caller ended answers the caller's error.
func Exec(ctx context.Context, req Request) (Execution, error) {
	run, cancel := ctx, context.CancelFunc(func() { /* no budget of its own: ctx alone ends the run */ })
	if req.Timeout > 0 {
		run, cancel = context.WithTimeout(ctx, req.Timeout)
	}
	defer cancel()
	var start time.Time
	if req.Clock != nil {
		start = req.Clock()
	}
	var stdin io.Reader
	if req.Stdin != nil {
		stdin = bytes.NewReader(req.Stdin)
	}
	out, err := runner.Run(run, runner.Command{Name: req.Path, Args: req.Args, Dir: req.Dir, Env: req.Env, Stdin: stdin})
	got := Execution{Output: out}
	if req.Clock != nil {
		got.Elapsed = req.Clock().Sub(start)
	}
	if killed(ctx, err) {
		got.Killed = true
		return got, nil
	}
	if err != nil {
		return got, fmt.Errorf("run %s: %w", filepath.Base(req.Path), err)
	}
	return got, nil
}

// killed tells the tool's own deadline apart from the caller's: only the
// first is a kill the event survives.
func killed(ctx context.Context, err error) bool {
	return err != nil && ctx.Err() == nil && errors.Is(err, context.DeadlineExceeded)
}

// ToolEnv reduces the environment to what M6.6.2 names: PATH, HOME,
// TMPDIR and the locale. TMP, TEMP, USERPROFILE and SYSTEMROOT are the
// Windows names of the same home and temporary directory and the variable
// every Windows program needs to start, so nothing a tool could read as
// configuration (NODE_OPTIONS, PYTHONPATH, proxies, RUFF_*) passes.
func ToolEnv(environ []string) []string {
	keep := []string{"PATH", "HOME", "TMPDIR", "TMP", "TEMP", "USERPROFILE", "SYSTEMROOT", "LANG", "LANGUAGE"}
	out := make([]string, 0, len(environ))
	for _, entry := range environ {
		name, _, found := strings.Cut(entry, "=")
		if !found {
			continue
		}
		// Windows compares variable names without case.
		upper := strings.ToUpper(name)
		if slices.Contains(keep, upper) || strings.HasPrefix(upper, "LC_") {
			out = append(out, entry)
		}
	}
	return out
}
