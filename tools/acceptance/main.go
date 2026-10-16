// Command acceptance checks installed hook invocations in an isolated home.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// collectedRows is the number of rows beyond the fixtures a run adds.
const (
	categoryDependency = "dependency"
	collectedRows      = 21
)

func main() { os.Exit(run(os.Stdout, runCommand, exec.LookPath)) }

func run(out io.Writer, execute executor, lookup locator) int {
	return runArgs(out, execute, lookup, os.Args[1:])
}

func runArgs(out io.Writer, execute executor, lookup locator, args []string) int {
	ctx := context.Background()
	flags := flag.NewFlagSet(dirAcceptance, flag.ContinueOnError)
	flags.SetOutput(out)
	required := flags.Bool("release", false, "require every release evidence row")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return 1
	}
	cwd, err := os.Getwd()
	if err != nil {
		return setupFailure(out, err)
	}
	root, err := moduleRoot(cwd)
	if err != nil {
		return setupFailure(out, err)
	}
	base, err := os.MkdirTemp("", "golaw-acceptance-")
	if err != nil {
		return setupFailure(out, fmt.Errorf("create temporary home: %w", err))
	}
	defer os.RemoveAll(base) //nolint:errcheck // Every path belongs to the disposable acceptance home.
	s := newSuite(root, base, execute, lookup)
	if err := s.prepare(ctx); err != nil {
		return setupFailure(out, err)
	}
	rows, version := s.collect(ctx, *required)
	if report(out, rows, version, s.digest) {
		return 0
	}
	return 1
}

func setupFailure(out io.Writer, err error) int {
	report(out, []row{measured("setup", "installation", err)}, "absent", "")
	return 1
}

func (s *suite) collect(ctx context.Context, required bool) ([]row, string) {
	items := fixtures()
	rows := make([]row, 0, len(items)+collectedRows)
	for i := range items {
		rows = append(rows, s.runFixture(ctx, items[i]))
	}
	rows = append(rows, aliasRow(ctx, s), s.doctorRow(ctx), s.existingRow(ctx), s.reenrollRow(ctx))
	rows = append(rows, s.crossRows(ctx)...)
	rows = append(rows, s.failureRows(ctx)...)
	vendors, version := s.vendorRows(ctx, required)
	rows = append(rows, vendors...)
	rows = append(rows, s.liveRows(ctx, required)...)
	rows = append(rows,
		row{categoryDependency, "internal-replace", statusSkip, "dependency checks enter in M3", false},
		row{categoryDependency, "external-replace", statusSkip, "dependency checks enter in M3", false},
		row{"adapter", "additional-patch-shapes", statusSkip, "additional adapters enter in M4", false},
	)
	return rows, version
}
