// Command vulngate reads the JSON stream of govulncheck on stdin and fails
// on every advisory reachable from the code (a finding whose trace reaches
// a function) unless golaw.yml carries an unexpired waiver for it. It is
// the exception path of the scheduled vulnerability lane: a waiver names
// the advisory, the day it expires and the decision record that justifies
// it; the lane prints every waiver it applied, and an expired waiver is red
// again so the list only shrinks. The tool reads no clock: the lane passes
// the date, so a run is reproducible.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"
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
	flags := flag.NewFlagSet("vulngate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	waiversPath := flags.String("waivers", "golaw.yml", "policy file holding vulnerability_waivers")
	today := flags.String("today", "", "the current date as YYYY-MM-DD, required")
	if err := flags.Parse(args); err != nil {
		return exitFailure
	}
	day, err := time.Parse(time.DateOnly, *today)
	if err != nil {
		return fail(stderr, fmt.Errorf("-today: %w", err))
	}
	waivers, err := loadWaivers(*waiversPath)
	if err != nil {
		return fail(stderr, err)
	}
	reachable, err := readReachable(stdin)
	if err != nil {
		return fail(stderr, err)
	}
	lines, red := judge(reachable, waivers, day)
	for _, line := range lines {
		if _, err := fmt.Fprintln(stdout, "vulngate:", line); err != nil {
			return exitFailure
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
