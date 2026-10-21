// Command vulngate fails on every advisory reachable from the code (a
// finding whose trace reaches a function) unless golaw.yml carries an
// unexpired waiver for it. It has two modes: it reads the JSON stream of
// govulncheck on stdin, or with -scan it runs the pinned govulncheck over
// ./... itself, so make vuln is one plain command on every system. It is
// the exception path of the vulnerability lane: a waiver names the
// advisory, the day it expires and the decision record that justifies it;
// the lane prints every waiver it applied, and an expired waiver is red
// again so the list only shrinks. Stdin mode requires -today; with -scan
// the main reads the UTC day of the clock when -today is absent, and an
// explicit -today keeps a run reproducible. The judgment is internal/vuln,
// the one the pre-push rung applies.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/vuln"
)

// The exit codes: nothing reachable or every reachable advisory waived, a
// reachable advisory without a valid waiver, the tool itself failed.
const (
	exitClean     = 0
	exitReachable = 1
	exitFailure   = 2
)

func main() {
	os.Exit(execute(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// execute is main without the process, so the tests reach every exit code
// in memory.
func execute(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	clock := time.Now //nolint:forbidigo // A main package is a composition root and reads the clock, as cmd does.
	return executeWithScan(context.Background(), args, stdin, stdout, stderr, scanDependencies{
		now: clock().UTC(), readOutput: runner.ReadOutput,
	})
}

// executeWithScan is execute with the clock and the scanner supplied, so
// the tests reach both modes and every scanner failure without a process.
func executeWithScan(
	ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, deps scanDependencies,
) int {
	opts, err := parseOptions(args, stderr, deps.now)
	if err != nil {
		return fail(stderr, err)
	}
	waivers, err := loadWaivers(opts.waiversPath)
	if err != nil {
		return fail(stderr, err)
	}
	var reachable []vuln.Finding
	if opts.scan {
		reachable, err = scanFindings(ctx, deps)
	} else {
		reachable, err = vuln.Reachable(stdin)
	}
	if err != nil {
		return fail(stderr, err)
	}
	lines, red := verdictLines(reachable, waivers, opts.day)
	for _, line := range lines {
		if _, err := fmt.Fprintln(stdout, "vulngate:", line); err != nil {
			return fail(stderr, fmt.Errorf("write verdict: %w", err))
		}
	}
	if red {
		return exitReachable
	}
	return exitClean
}

// fail returns exitFailure even when stderr refuses the message, because
// the exit code is the only signal the lane reads.
func fail(stderr io.Writer, err error) int {
	if _, writeErr := fmt.Fprintln(stderr, "vulngate:", err); writeErr != nil {
		return exitFailure
	}
	return exitFailure
}
