// The helpers every stage test shares: real repositories from gittest, the
// policy in force loaded with an empty state directory, and the catalog.
package stage_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/checks"
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/source"
	"github.com/cristianargotti/golaw/internal/stage"
)

// dash is the em dash the dash law refuses, spelled by code point so this
// file holds none.
var dash = string(rune(0x2014))

// discover adapts the source discovery, as cmd does.
func discover(ctx context.Context, dir string) (policy.Discovery, error) {
	repo, err := source.Discover(ctx, dir)
	if err != nil {
		return policy.Discovery{}, fmt.Errorf("discover: %w", err)
	}
	return policy.Discovery{Root: repo.Root, Head: repo.Head}, nil
}

// schemas are the option schemas of the catalog, as cmd builds them.
func schemas() policy.Schemas {
	out := policy.Schemas{}
	for _, c := range checks.Catalog() {
		if spec := c.Spec(); len(spec.Options) > 0 {
			out[spec.ID] = spec.Options
		}
	}
	return out
}

// loaded is the policy in force of the repository at root, for a
// development build, with an empty state directory: unenrolled.
func loaded(t *testing.T, root string) *policy.Loaded {
	t.Helper()
	base := t.TempDir()
	l, err := policy.Load(t.Context(), policy.Input{
		Dir: root, Dirs: policy.Dirs{State: filepath.Join(base, "state"), Config: filepath.Join(base, "config")},
		Now: time.Date(2026, time.October, 19, 12, 0, 0, 0, time.UTC), Schemas: schemas(), Discover: discover,
	})
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// input is the staged scan input of a repository with the catalog.
func input(t *testing.T, r *gittest.Repo, opts source.IndexOptions) stage.Input {
	t.Helper()
	repo, err := source.Discover(t.Context(), r.Root)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := checks.Registry()
	if err != nil {
		t.Fatal(err)
	}
	return stage.Input{Loaded: loaded(t, r.Root), Repo: repo, Index: opts, Registry: registry, Schemas: schemas()}
}

// scan is the staged scan of a repository, which must not fail.
func scan(t *testing.T, r *gittest.Repo, opts source.IndexOptions) stage.Result {
	t.Helper()
	result, err := stage.Scan(t.Context(), input(t, r, opts))
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// findingPaths lists the path of every finding of a decision, in order.
func findingPaths(d engine.Decision) []string {
	out := make([]string, 0, len(d.Findings))
	for _, f := range d.Findings {
		out = append(out, f.Path)
	}
	return out
}
