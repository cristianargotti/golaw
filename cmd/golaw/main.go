// Command golaw is the Go engineering standards kit: the scan over a tree,
// the agent hooks, the setup commands and the exit codes. This file is the
// composition root: it binds every seam of the app to its production
// implementation, and nothing else in the package knows a concrete one.
package main

import (
	"context"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/cristianargotti/golaw/internal/buildinfo"
	"github.com/cristianargotti/golaw/internal/checks"
	"github.com/cristianargotti/golaw/internal/engine"
)

func main() {
	os.Exit(newApp(os.Stdout, os.Stderr).run(context.Background(), os.Args[1:]))
}

// newApp binds the production implementations of the seams the commands
// run through; a test builds an app with fakes instead.
func newApp(stdout, stderr io.Writer) *app {
	b := binding{
		goos: runtime.GOOS,
		env:  os.Getenv,
		home: os.UserHomeDir,
		info: buildinfo.Current(),
	}
	return &app{
		stdout:    stdout,
		stderr:    stderr,
		buildInfo: buildinfo.Current,
		now:       time.Now,
		home:      os.UserHomeDir,
		registry:  func() (*engine.Registry, error) { return checks.Registry() },
		snapshot:  b.snapshot,
		load:      b.load,
		explain:   b.explain,
		render:    b.render,
	}
}
