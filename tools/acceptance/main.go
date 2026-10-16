// Command acceptance checks installed hook invocations in an isolated home.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

func main() { os.Exit(run(os.Stdout, runCommand, exec.LookPath)) }

// run fails its setup row until the installation rows of the harness
// exist, so the process and report assertions alone never pass a run.
func run(out io.Writer, _ executor, _ locator) int {
	return setupFailure(out, fmt.Errorf("installation rows absent: %w", ErrAcceptance))
}

func setupFailure(out io.Writer, err error) int {
	report(out, []row{measured("setup", "installation", err)}, "absent", "")
	return 1
}
