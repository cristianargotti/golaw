// Command depsage refuses a module of the build graph that entered too
// early: a version enters seven days after publication (section 3 of the
// specification), and a module without a publication time fails, because
// an unknown age is not old enough. The times are the ones the module proxy
// recorded, read through go list -m -json all. A version that must enter
// sooner, a security update, is named with -allow beside its decision
// record; the thirty days of a major update are the Dependabot cooldown's.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

// The exit codes: every module old enough, a module that entered too early,
// the tool itself failed.
const (
	exitClean   = 0
	exitYoung   = 1
	exitFailure = 2
	defaultDays = 7
	hoursPerDay = 24
	day         = hoursPerDay * time.Hour
)

// lister prints the module graph as the go command does.
type lister func(context.Context) ([]byte, error)

func main() {
	clock := time.Now //nolint:forbidigo // A main package is a composition root and reads the clock, as cmd does.
	os.Exit(execute(context.Background(), os.Args[1:], os.Stdout, os.Stderr, goList, clock))
}

// goList runs the go command over the module graph of the working directory.
func goList(ctx context.Context) ([]byte, error) {
	out, err := exec.CommandContext(ctx, "go", "list", "-m", "-json", "all").Output()
	if err != nil {
		return nil, fmt.Errorf("%w: go list -m -json all: %w", ErrStream, err)
	}
	return out, nil
}

// execute parses the flags, judges the graph and returns the exit code.
func execute(ctx context.Context, args []string, stdout, stderr io.Writer, list lister, clock func() time.Time) int {
	flags := flag.NewFlagSet("depsage", flag.ContinueOnError)
	flags.SetOutput(stderr)
	minimum := flags.Int("days", defaultDays, "minimum age of a module version, in days")
	var allowed []string
	flags.Func("allow", "a path@version that enters before the minimum age, beside its decision record",
		func(id string) error {
			allowed = append(allowed, id)
			return nil
		})
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *minimum < 1 {
		return exitFailure
	}
	stream, err := list(ctx)
	if err != nil {
		return fail(stderr, err)
	}
	modules, err := decode(stream)
	if err != nil {
		return fail(stderr, err)
	}
	lines, judged := judge(modules, clock(), time.Duration(*minimum)*day, allowed)
	for _, line := range lines {
		if _, err := fmt.Fprintln(stdout, "deps-age:", line); err != nil {
			return exitFailure
		}
	}
	if len(lines) != 0 {
		return exitYoung
	}
	if _, err := fmt.Fprintf(stdout, "deps-age: %d judged, each published at least %d days ago\n", judged, *minimum); err != nil {
		return exitFailure
	}
	return exitClean
}

// fail reports a failure of the tool; the exit code is 2 even when stderr
// refuses the message, since no channel is left to report that on.
func fail(stderr io.Writer, err error) int {
	if _, writeErr := fmt.Fprintln(stderr, "deps-age:", err); writeErr != nil {
		return exitFailure
	}
	return exitFailure
}
