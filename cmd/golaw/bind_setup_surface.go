package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// fetchTimeout bounds one download; the largest pinned asset is tens of
// megabytes, and a slow link still finishes well inside it.
const fetchTimeout = 10 * time.Minute

// setupSurface installs the tools of the languages the repository holds,
// or every tool with all, into the per-OS tool directory.
func (b binding) setupSurface(ctx context.Context, out io.Writer, path string, opts surfaceSetup) error {
	root, err := b.discover(ctx, path)
	if err != nil {
		return err
	}
	dirs, err := b.dirs()
	if err != nil || dirs.Tools == "" {
		return fmt.Errorf("%w: no tools directory resolves on this machine: set XDG_DATA_HOME or HOME", model.ErrUsage)
	}
	snapshot, err := b.snapshot(ctx, root)
	if err != nil {
		return fmt.Errorf("read the surface of %s: %w", root, err)
	}
	inv, err := surface.Take(ctx, snapshot)
	if err != nil {
		return fmt.Errorf("read the surface of %s: %w", root, err)
	}
	if len(inv.Languages) == 0 && !opts.all {
		return writeLines(out, []string{fmt.Sprintf("nenhuma linguagem de superfície em %s: nada a instalar", root)})
	}
	_, err = toolchain.Install(ctx, toolchain.InstallOptions{
		Dir: dirs.Tools, GOOS: b.goos, GOARCH: runtime.GOARCH, Languages: inv.Languages, All: opts.all,
		Fetch: opts.fetch, Run: runner.Run, Env: b.environ(), Out: out, Tools: opts.tools,
	})
	if err != nil {
		return fmt.Errorf("install the surface tools: %w", err)
	}
	return nil
}

// httpFetch is the one place of the kit that reaches the network (A29):
// golaw setup --surface, run by a person, downloads the pinned assets the
// installer then verifies by checksum.
func httpFetch(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("%w: GET %s: %w", toolchain.ErrFetch, url, err)
	}
	resp, err := (&http.Client{Timeout: fetchTimeout}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: GET %s: %w", toolchain.ErrFetch, url, err)
	}
	if resp.StatusCode != http.StatusOK {
		// The body of a refusal carries nothing the person needs beyond the
		// status; a failed close leaves nothing to undo.
		_ = resp.Body.Close()
		return nil, fmt.Errorf("%w: GET %s answered %s", toolchain.ErrFetch, url, resp.Status)
	}
	// Install closes the body it reads.
	return resp.Body, nil
}
