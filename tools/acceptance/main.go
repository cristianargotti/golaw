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
	collectedRows      = 23
)

// The words and error formats the M3 and M4 rows share: red, the side of a
// fixture and the state of a doctor line, the rules directory and key, and
// the failures of removing or encoding a file.
const (
	redWord      = "red"
	rulesWord    = "rules"
	removeFailed = "remove %s: %w"
	encodeFailed = "encode %s: %w"
)

// The git status words the rows share: the porcelain status names untracked
// files too, so a step that left any new file in a repository is seen.
const (
	gitStatus    = "status"
	gitPorcelain = "--porcelain"
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
	onlyM2 := flags.Bool("m2", false, "require M2 functional evidence rows")
	onlyM3 := flags.Bool("m3", false, "require M3 functional evidence rows")
	modes := m4FlagSet(flags)
	m5Modes := m5FlagSet(flags)
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
	if code, routed := modes.route(ctx, out, s, *required); routed {
		return code
	}
	if code, routed := m5Modes.route(ctx, out, s, *required); routed {
		return code
	}
	if *onlyM3 && !*required {
		return s.runM3(ctx, out)
	}
	return s.runAcceptance(ctx, out, *required, *onlyM2)
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
	rows = append(rows, s.projectRows(ctx)...)
	rows = append(rows, s.crossRows(ctx)...)
	rows = append(rows, s.failureRows(ctx)...)
	vendors, version := s.vendorRows(ctx, required)
	rows = append(rows, vendors...)
	rows = append(rows, s.liveRows(ctx, required)...)
	rows = append(rows, s.dependencyRows(ctx)...)
	return rows, version
}
