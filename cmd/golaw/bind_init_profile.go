package main

import (
	"context"
	"fmt"
	"time"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
)

// initBundle loads the data of the bundle the policy init proposes pins,
// through the loader every governed load binds (D6), so init refuses a
// missing, stale, mismatched or invalid bundle with the one error that
// names the install command (A19, D8) before it renders a template; a
// policy without a profile loads nothing.
func (b binding) initBundle(config policy.File) (*policy.Loaded, error) {
	loaded := &policy.Loaded{File: config}
	if config.Profile == nil {
		return loaded, nil
	}
	dirs, err := b.dirs()
	if err != nil {
		return nil, err
	}
	if loaded.Data, err = profile.Load(dirs.Config, *config.Profile); err != nil {
		return nil, fmt.Errorf("load the profile %s pins: %w", bootstrap.PolicyFile, err)
	}
	return loaded, nil
}

// profileTemplateRules answers what init takes from the profile data: the
// destinations it leaves to another owner (A1) and the layout defaults;
// neither for core only.
func profileTemplateRules(loaded *policy.Loaded) ([]string, *model.Layout) {
	if loaded == nil || loaded.Data == nil {
		return nil, nil
	}
	return loaded.Data.Templates.Exclude, loaded.Data.Layout
}

// seedPolicy is the policy the seed scan runs under: the policy init
// writes, compiled with the data of the bundle it pins, so the seed
// records what the checks and the terms of the profile find too.
func (b binding) seedPolicy(data []byte, now time.Time) (*model.EffectivePolicy, error) {
	f, err := policy.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("decode the policy to seed: %w", err)
	}
	loaded, err := b.initBundle(f)
	if err != nil {
		return nil, err
	}
	p, err := policy.CompileWith(f, now, schemas(), loaded.Data)
	if err != nil {
		return nil, fmt.Errorf("compile the policy to seed: %w", err)
	}
	return p, nil
}

// initProfile resolves --profile to the pin (resolveProfile) and reads the
// layout defaults of the bundle it pins, which a new policy renders where
// the tree shows no layout of its own (D14); a bundle that does not load is
// the one error of D8, before any file of the checkout is read.
func (a *app) initProfile(ctx context.Context, path string, opts *initOptions) error {
	if err := a.resolveProfile(ctx, path, opts); err != nil || opts.pin == nil {
		return err
	}
	loaded, err := a.bind().initBundle(policy.File{Profile: opts.pin})
	if err != nil {
		return err
	}
	_, opts.layout = profileTemplateRules(loaded)
	return nil
}
