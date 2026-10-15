// Command coverage reads a Go cover profile and fails when the total, any
// package or any file is under the floor (80 percent by default), skipping
// a package or a file with fewer statements than the minimum (20 by
// default): the coverage law of the kit until golaw coverage ships. Blocks
// reported by several test binaries count once, covered when any run
// executed them; a source file with statements that is absent from the
// profile fails on its own, whatever its size, so an untested package
// cannot pass unnoticed. There is no exemption list.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
)

// The exit codes: every floor met, a floor missed, the tool itself failed.
const (
	exitMet     = 0
	exitMissed  = 1
	exitFailure = 2
)

// The defaults of the kit's coverage law (specification section 3).
const (
	defaultFloor         = 80
	defaultMinStatements = 20
)

func main() {
	os.Exit(execute(os.Args[1:], os.Stdout, os.Stderr))
}

// execute parses the flags, evaluates the profile and returns the exit code.
func execute(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("coverage", flag.ContinueOnError)
	flags.SetOutput(stderr)
	profile := flags.String("profile", "coverage.out", "cover profile written by go test")
	root := flags.String("root", ".", "module root holding go.mod")
	floor := flags.Float64("min", defaultFloor, "minimum percentage in total, per package and per file")
	statements := flags.Int("min-statements", defaultMinStatements,
		"a package or file with fewer statements is not held to the floor")
	if err := flags.Parse(args); err != nil {
		return exitFailure
	}
	rep, err := run(*profile, *root, floors{percent: *floor, statements: *statements})
	if err != nil {
		if _, writeErr := fmt.Fprintln(stderr, "coverage:", err); writeErr != nil {
			return exitFailure
		}
		return exitFailure
	}
	if _, err := io.WriteString(stdout, rep.String()); err != nil {
		return exitFailure
	}
	if len(rep.Failures) > 0 {
		if _, err := fmt.Fprintf(stderr, "coverage: %d below %.0f%%\n", len(rep.Failures), *floor); err != nil {
			return exitFailure
		}
		return exitMissed
	}
	return exitMet
}

// run parses the profile, lists the source files that need coverage and
// evaluates both against the floors.
func run(profilePath, root string, f floors) (report, error) {
	data, err := os.ReadFile(profilePath) //nolint:gosec // the profile path is a flag of this tool
	if err != nil {
		return report{}, fmt.Errorf("read profile: %w", err)
	}
	files, err := parseProfile(bytes.NewReader(data))
	if err != nil {
		return report{}, err
	}
	module, err := modulePath(root)
	if err != nil {
		return report{}, err
	}
	sources, err := sourceFiles(root, module)
	if err != nil {
		return report{}, err
	}
	return evaluate(files, sources, f), nil
}
