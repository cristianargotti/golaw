package goldentest

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gen/apply"
	"github.com/cristianargotti/golaw/internal/gen/contract"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/source"
)

// snapshot answers a fresh worktree snapshot of the scratch module, so
// each step reads the files as the run left them.
func snapshot(ctx context.Context, root string) (model.Snapshot, error) {
	repo, err := source.Discover(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("discover %s: %w", root, err)
	}
	w, err := source.NewWorktree(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", root, err)
	}
	return w, nil
}

// input is the engine input golaw new would build over the module now;
// Main stays empty, the root layout of the fixture's golaw.yml.
func (h *harness) input(ctx context.Context) (gen.Input, error) {
	snap, err := snapshot(ctx, h.repo.Root)
	if err != nil {
		return gen.Input{}, err
	}
	return gen.Input{
		Module: h.s.Module, Version: h.policy.KitVersion, Policy: h.policy.Generate,
		Snapshot: snap, Manifest: h.manifest, Contracts: contract.Reader{Snapshot: snap},
		Kinds: h.s.Kinds,
	}, nil
}

// generate runs the commands in order, each over the tree the one before
// it wrote, and reads back every file they wrote; it answers false when a
// command failed, because no later step can judge a partial slice.
func (h *harness) generate(ctx context.Context) bool {
	h.manifest = manifest.Manifest{Schema: manifest.Schema}
	var written []string
	for i := range h.s.Commands {
		c := &h.s.Commands[i]
		where := fmt.Sprintf("command %s %s", c.Kind, c.Request.Name.String())
		paths, ok := h.command(ctx, where, c)
		if !ok {
			return false
		}
		written = append(written, paths...)
	}
	for _, path := range written {
		content, err := os.ReadFile(source.OSPath(h.repo.Root, path))
		if err != nil {
			h.failf(sliceWhere, "read back %s: %v", path, err)
			return false
		}
		h.report.Files[path] = content
	}
	return true
}

// command plans one golaw new and lands its writes.
func (h *harness) command(ctx context.Context, where string, c *Command) ([]string, bool) {
	in, err := h.input(ctx)
	if err != nil {
		h.failf(where, "%v", err)
		return nil, false
	}
	result, err := gen.New(ctx, in, c.Kind, c.Request)
	if err != nil {
		h.failf(where, "%v", err)
		return nil, false
	}
	landed, err := apply.Files(h.repo.Root, result.Writes)
	if err != nil {
		h.failf(where, "%v", err)
		return nil, false
	}
	if len(landed.Skipped) != 0 {
		skipped := make([]string, 0, len(landed.Skipped))
		for _, s := range landed.Skipped {
			skipped = append(skipped, s.Path+" "+s.Cause)
		}
		h.failf(where, "writes skipped: %s", strings.Join(skipped, "; "))
		return nil, false
	}
	h.manifest = result.Manifest
	return landed.Written, true
}
