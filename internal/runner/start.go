package runner

import (
	"context"
	"io"
	"os/exec"
)

// Process is a command Start began and Wait ends, so a caller runs several
// tools at once as processes, with no goroutine of its own to leak.
type Process struct {
	cmd            *exec.Cmd
	name           string
	stdout, stderr *bounded
}

// Start begins one command under ctx and answers at once. Its standard
// output goes to stdout when one is given, a file for a stream of any size,
// and is otherwise kept up to MaxOutputBytes as Run keeps it. A program
// that cannot start answers what Run answers for it.
func Start(ctx context.Context, c Command, stdout io.Writer) (*Process, error) {
	cmd := exec.CommandContext(ctx, c.Name, c.Args...) //nolint:gosec // the kit names the program and its arguments
	cmd.Dir, cmd.Env, cmd.Stdin, cmd.WaitDelay = c.Dir, c.Env, c.Stdin, waitDelay
	p := &Process{cmd: cmd, name: c.Name, stdout: &bounded{limit: MaxOutputBytes}, stderr: &bounded{limit: MaxOutputBytes}}
	cmd.Stdout, cmd.Stderr = p.stdout, p.stderr
	if stdout != nil {
		cmd.Stdout = stdout
	}
	if err := cmd.Start(); err != nil {
		_, startErr := outcome(ctx, c.Name, Output{}, err)
		return nil, startErr
	}
	return p, nil
}

// Wait waits for the process to end and answers its Output the way Run
// does; ctx is the context Start ran it under.
func (p *Process) Wait(ctx context.Context) (Output, error) {
	err := p.cmd.Wait()
	return outcome(ctx, p.name, Output{Stdout: p.stdout.buf.Bytes(), Stderr: p.stderr.buf.Bytes()}, err)
}
