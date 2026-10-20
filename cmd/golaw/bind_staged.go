package main

import (
	"context"
	"fmt"
	"time"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/source"
	"github.com/cristianargotti/golaw/internal/stage"
)

// staged is the staged scan of the repository that holds the directory of
// in: the policy in force (the pin of A10 and the enrollment checked, as for
// every scan; before the adoption commit of a repository with a history,
// the enrolled golaw.yml the index holds, Uncommitted), the repository its
// discovery found and the index of the process environment, judged by
// stage.Scan. The golaw.yml of the checkout is not asked for, because the
// next commit records the index, where a staged deletion is refused
// instead.
func (b binding) staged(ctx context.Context, in policy.Input, registry *engine.Registry) (stage.Result, error) {
	in.Uncommitted = true
	var repo source.Repo
	in.Discover = func(ctx context.Context, dir string) (policy.Discovery, error) {
		found, err := source.Discover(ctx, dir)
		if err != nil {
			return policy.Discovery{}, discoverFailed(dir, err)
		}
		repo = found
		return policy.Discovery{Root: found.Root, Head: found.Head}, nil
	}
	l, err := b.loadedBy(ctx, in)
	if err != nil {
		return stage.Result{}, err
	}
	result, err := stage.Scan(ctx, stage.Input{Loaded: l, Repo: repo, Registry: registry, Schemas: schemas()})
	if err != nil {
		return stage.Result{}, fmt.Errorf("scan the index of %s: %w", repo.Root, err)
	}
	return result, nil
}

// dirsFor fills the directories the load of in reads. The enrollment a
// pipeline provisions (A3) reads no record, so its load resolves no state
// directory, and only a policy that names a profile reads the config
// directory: a runner whose environment names neither is refused only by
// a load that reads one.
func (b binding) dirsFor(in *policy.Input) error {
	if in.Expected == nil {
		dirs, err := b.dirs()
		in.Dirs = dirs
		return err
	}
	config, err := policy.ConfigDir(b.goos, b.env)
	if err != nil {
		in.ConfigErr = fmt.Errorf("resolve the config directory: %w", err)
	}
	in.Dirs = policy.Dirs{Config: config}
	return nil
}

// expectedLoad is the load seam under the enrollment the pipeline
// provisions (A3): the policy in force of the repository that holds root,
// enrolled with the expected digests instead of the record of the state
// directory, and refused, as the load seam refuses it, when the checkout
// no longer holds golaw.yml.
func (b binding) expectedLoad(expected *policy.Expected) func(ctx context.Context, root string, now time.Time) (*model.EffectivePolicy, error) {
	return func(ctx context.Context, root string, now time.Time) (*model.EffectivePolicy, error) {
		l, err := b.loadedBy(ctx, policy.Input{Dir: root, Now: now, Discover: locate, Expected: expected})
		if err != nil {
			return nil, err
		}
		if err := enrolledCheckout(l); err != nil {
			return nil, err
		}
		return l.Policy, nil
	}
}
