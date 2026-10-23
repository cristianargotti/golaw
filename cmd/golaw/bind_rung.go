package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
	"github.com/cristianargotti/golaw/internal/rung"
)

// rungDependencies binds a git rung to the production adapters: the policy
// in force for the next commit (policy.LoadNext: the pin of A10 for a
// released build, the enrollment, the enrolled golaw.yml of the index
// before the adoption commit and the staged pin of a kit upgrade, an
// enrolled checkout without golaw.yml refused), the whole environment git
// gave the hook, the schemas of the catalog and the state directory where
// the copies of trees live.
func (b binding) rungDependencies(dir string, now time.Time, stdin io.Reader, registry *engine.Registry) (rung.Dependencies, error) {
	dirs, err := b.dirs()
	if err != nil {
		return rung.Dependencies{}, err
	}
	load := func(ctx context.Context, dir string) (*policy.Loaded, error) {
		in := policy.Input{Dir: dir, Dirs: dirs, Version: b.pin(), Now: now, Schemas: schemas(), Discover: locate, Bundle: profile.Load}
		l, err := policy.LoadNext(ctx, in)
		if err != nil {
			return nil, fmt.Errorf("load the policy of %s: %w", dir, err)
		}
		if err := enrolledCheckout(l); err != nil {
			return nil, err
		}
		return l, nil
	}
	return rung.Dependencies{
		Dir: dir, Environ: b.environ(), Now: now, Load: load,
		Registry: registry, Schemas: schemas(), State: dirs.State, Stdin: stdin,
	}, nil
}
