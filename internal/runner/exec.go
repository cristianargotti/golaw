package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// MaxOutputBytes bounds what Run keeps of each stream; the rest is read and
// dropped, so a tool that prints without end never fills the memory of a
// rung and never blocks on a full pipe.
const MaxOutputBytes = 4 << 20

// waitDelay is how long Run waits for the pipes of a process the context
// stopped, because a child of the process may hold them open.
const waitDelay = time.Second

// Command is one process the kit runs.
type Command struct {
	Name  string    // the program; a name without a path separator is looked up in PATH
	Args  []string  // the arguments after the program
	Dir   string    // the working directory
	Env   []string  // the whole environment of the process; nil inherits the kit's
	Stdin io.Reader // nil gives the process an empty input
}

// Output is what a process answered.
type Output struct {
	Stdout []byte // at most MaxOutputBytes
	Stderr []byte // at most MaxOutputBytes
	Code   int    // the exit status
}

// Run runs one command to its end under ctx. A program that cannot start
// is ErrStart naming it; a context that ends first stops the process,
// waits at most a second for its pipes, and answers the context's error;
// any exit status is an Output without an error, so the caller reads Code.
func Run(ctx context.Context, c Command) (Output, error) {
	cmd := exec.CommandContext(ctx, c.Name, c.Args...) //nolint:gosec // the kit names the program and its arguments
	cmd.Dir, cmd.Env, cmd.Stdin, cmd.WaitDelay = c.Dir, c.Env, c.Stdin, waitDelay
	stdout, stderr := &bounded{limit: MaxOutputBytes}, &bounded{limit: MaxOutputBytes}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err := cmd.Run()
	out := Output{Stdout: stdout.buf.Bytes(), Stderr: stderr.buf.Bytes()}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return out, fmt.Errorf("run %s: %w", c.Name, ctxErr)
	}
	// exec answers ErrWaitDelay for a program that exited 0 while a
	// descendant still held its pipes: the program ran, and the bounded wait
	// only closed the pipes it left behind.
	if err == nil || errors.Is(err, exec.ErrWaitDelay) {
		return out, nil
	}
	if exit, exited := errors.AsType[*exec.ExitError](err); exited {
		out.Code = exit.ExitCode()
		return out, nil
	}
	return out, fmt.Errorf("%w: %s: %w", ErrStart, c.Name, err)
}

// bounded keeps the first limit bytes written to it and drops the rest
// while reporting every byte written, so the process never sees a short
// write.
type bounded struct {
	buf   bytes.Buffer
	limit int
}

func (b *bounded) Write(p []byte) (int, error) {
	room := b.limit - b.buf.Len()
	if room <= 0 {
		return len(p), nil
	}
	if _, err := b.buf.Write(p[:min(room, len(p))]); err != nil {
		return 0, fmt.Errorf("keep the output: %w", err)
	}
	return len(p), nil
}
