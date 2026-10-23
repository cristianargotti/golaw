// Command releasegate decides whether the commit of a tag may be released
// (D12): the last run of ci.yml for that commit on main by push must have
// completed with success. It reads the answer of the GitHub runs endpoint
// from a file, so the release lane and a person replaying the decision
// with make release-gate-dry-run judge the same bytes the same way. The
// exit codes: 0 the commit may be released, 1 it may not, 2 the tool could
// not decide (usage, an unreadable or foreign file).
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
)

// The exit codes of the gate.
const (
	exitReleasable = 0
	exitRefused    = 1
	exitFailure    = 2
)

// commitPattern is a full commit id: SHA-1 or SHA-256, lower case hex.
var commitPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

func main() {
	os.Exit(execute(os.Args[1:], os.Stdout, os.Stderr))
}

// execute is main without the process, so the tests reach every exit code
// in memory.
func execute(args []string, stdout, stderr io.Writer) int {
	sha, path, err := parseOptions(args, stderr)
	if err != nil {
		return fail(stderr, err)
	}
	runs, err := os.ReadFile(path) //nolint:gosec // the lane or the person names the file of runs
	if err != nil {
		return fail(stderr, fmt.Errorf("%w: %w", errRuns, err))
	}
	ok, reason, err := Decide(runs, sha)
	if err != nil {
		return fail(stderr, err)
	}
	if _, err := fmt.Fprintln(stdout, "releasegate:", reason); err != nil {
		return fail(stderr, fmt.Errorf("write the decision: %w", err))
	}
	if !ok {
		return exitRefused
	}
	return exitReleasable
}

// parseOptions requires -sha, a full commit id, and -runs, the file.
func parseOptions(args []string, stderr io.Writer) (string, string, error) {
	flags := flag.NewFlagSet("releasegate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	sha := flags.String("sha", "", "the full commit id of the tag")
	runs := flags.String("runs", "", "the file holding the answer of the runs endpoint of ci.yml")
	if err := flags.Parse(args); err != nil {
		return "", "", fmt.Errorf("%w: %w", errUsage, err)
	}
	switch {
	case !commitPattern.MatchString(*sha):
		return "", "", fmt.Errorf("%w: -sha %q is not a full lower case commit id", errUsage, *sha)
	case *runs == "":
		return "", "", fmt.Errorf("%w: -runs names no file", errUsage)
	}
	return *sha, *runs, nil
}

// fail prints the cause and answers exitFailure even when stderr refuses
// the line, because the exit code is the signal the lane reads.
func fail(stderr io.Writer, err error) int {
	if _, writeErr := fmt.Fprintln(stderr, "releasegate:", err); writeErr != nil {
		return exitFailure
	}
	return exitFailure
}
