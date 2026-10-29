package main

import (
	"context"
	"fmt"
	"io"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// The --surface flag and its values: the tools of the languages the
// repository holds, or every tool of the table.
const (
	surfaceFlag    = "surface"
	surfacePresent = "present"
	surfaceAll     = "all"
)

// surfaceSetup is one run of golaw setup --surface.
type surfaceSetup struct {
	all, digest bool
	fetch       toolchain.Fetcher
	tools       []toolchain.Tool // nil installs the table; a test passes its own
}

// surfaceArgs reads the value of --surface and the path. A bare --surface
// followed by all reads all as the value, because cobra cannot take an
// optional value after a space; a directory named all is passed as ./all.
func surfaceArgs(args []string, value string, digest bool) (string, surfaceSetup, error) {
	opts := surfaceSetup{digest: digest}
	switch value {
	case surfaceAll:
		opts.all = true
	case surfacePresent:
		if len(args) == 1 && args[0] == surfaceAll {
			opts.all = true
			return ".", opts, nil
		}
	default:
		return "", surfaceSetup{}, fmt.Errorf("%w: --surface takes present or all, not %q", model.ErrUsage, value)
	}
	return rootArg(args), opts, nil
}

// setupSurfaceCommand installs the pinned surface linters, or prints the
// toolchain digest a CI cache keys on. It is run by a person and is the
// only path of the kit that downloads anything (M6.5.2).
func (a *app) setupSurfaceCommand(ctx context.Context, out io.Writer, args []string, value string, digest, uninstall bool) error {
	switch {
	case digest && value == "":
		return setupFailed(fmt.Errorf("%w: --print-digest needs --surface", model.ErrUsage))
	case uninstall:
		return setupFailed(fmt.Errorf("%w: --surface and --uninstall do not combine: the tools stay in the tool directory", model.ErrUsage))
	}
	path, opts, err := surfaceArgs(args, value, digest)
	if err != nil {
		return setupFailed(err)
	}
	if opts.digest {
		return setupFailed(writeLines(out, []string{toolchain.Digest()}))
	}
	opts.fetch = httpFetch
	return setupFailed(a.bind().setupSurface(ctx, out, path, opts))
}
