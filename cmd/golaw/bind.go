package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cristianargotti/golaw/internal/buildinfo"
	"github.com/cristianargotti/golaw/internal/checks"
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/report"
	"github.com/cristianargotti/golaw/internal/source"
)

// binding adapts the source, policy and report packages to the seams of
// app. It resolves the directories outside the checkout from the
// environment it was given and never reads the process environment
// itself, so a test binds it to a temporary home.
type binding struct {
	goos    string
	env     func(string) string
	environ func() []string
	home    func() (string, error)
	info    buildinfo.Info
}

// identified is what a snapshot of the source package answers beyond the
// model interface: the identity the report prints.
type identified interface{ Identity() string }

// pin is the kit version the policy pin of A10 is checked against: empty
// for a development build, which skips the pin.
func (b binding) pin() string {
	release, _ := b.info.Release()
	return release
}

// dirs resolves the per-OS directories outside the checkout.
func (b binding) dirs() (policy.Dirs, error) {
	dirs, err := policy.DefaultDirs(b.goos, b.env)
	if err != nil {
		return policy.Dirs{}, fmt.Errorf("resolve the state directories: %w", err)
	}
	return dirs, nil
}

// installState is the state directory the installer and doctor share: the
// per-OS state directory of the enrollment record, so the receipt, the
// observation and the record of one repository live in one tree.
func (b binding) installState() (string, error) {
	dirs, err := b.dirs()
	if err != nil {
		return "", err
	}
	return dirs.State, nil
}

// discover answers the real root of the repository that holds a directory.
func (b binding) discover(ctx context.Context, dir string) (string, error) {
	repo, err := source.Discover(ctx, dir)
	if err != nil {
		return "", discoverFailed(dir, err)
	}
	return repo.Root, nil
}

// discoverFailed names the directory whose repository was not found.
func discoverFailed(dir string, err error) error {
	return fmt.Errorf("discover %s: %w", dir, err)
}

// locate adapts the source discovery to the policy locator.
func locate(ctx context.Context, dir string) (policy.Discovery, error) {
	repo, err := source.Discover(ctx, dir)
	if err != nil {
		return policy.Discovery{}, discoverFailed(dir, err)
	}
	return policy.Discovery{Root: repo.Root, Head: repo.Head}, nil
}

// schemas are the option schemas of the catalog, keyed by check id, so the
// policy validates the options a check declares.
func schemas() policy.Schemas {
	out := policy.Schemas{}
	for _, c := range checks.Catalog() {
		if spec := c.Spec(); len(spec.Options) > 0 {
			out[spec.ID] = spec.Options
		}
	}
	return out
}

// loaded establishes the policy in force for a directory.
func (b binding) loaded(ctx context.Context, dir string, now time.Time) (*policy.Loaded, error) {
	dirs, err := b.dirs()
	if err != nil {
		return nil, err
	}
	in := policy.Input{Dir: dir, Dirs: dirs, Version: b.pin(), Now: now, Schemas: schemas(), Discover: locate}
	l, err := policy.Load(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("load the policy of %s: %w", dir, err)
	}
	return l, nil
}

// load is the load seam: the effective policy in force.
func (b binding) load(ctx context.Context, root string, now time.Time) (*model.EffectivePolicy, error) {
	l, err := b.loaded(ctx, root, now)
	if err != nil {
		return nil, err
	}
	return l.Policy, nil
}

// snapshot is the snapshot seam: the frozen worktree universe of the
// repository that holds root.
func (b binding) snapshot(ctx context.Context, root string) (model.Snapshot, error) {
	repo, err := source.Discover(ctx, root)
	if err != nil {
		return nil, discoverFailed(root, err)
	}
	w, err := source.NewWorktree(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("freeze the worktree of %s: %w", repo.Root, err)
	}
	return w, nil
}

// compile is the compile seam: the rendered bytes of init decoded strictly
// and compiled against the catalog schemas, so a policy the kit cannot load
// is never written.
func (b binding) compile(data []byte, now time.Time) (*model.EffectivePolicy, error) {
	f, err := policy.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("decode the rendered policy: %w", err)
	}
	p, err := policy.Compile(f, now, schemas())
	if err != nil {
		return nil, fmt.Errorf("compile the rendered policy: %w", err)
	}
	return p, nil
}

// enroll is the enroll seam: the record of A3 written by init --apply,
// with the bytes written as the bootstrap policy of an unborn branch.
func (b binding) enroll(root string, data []byte, p *model.EffectivePolicy, now time.Time) error {
	dirs, err := b.dirs()
	if err != nil {
		return err
	}
	f, err := policy.Decode(data)
	if err != nil {
		return fmt.Errorf("decode the written policy: %w", err)
	}
	record := policy.Record{
		Root: root, PolicyDigest: p.Digest, ProfileDigest: profileDigest(f), KitVersion: p.KitVersion,
		Bootstrap: string(data), WrittenBy: policy.WrittenByInit, WrittenAt: now,
	}
	if err := policy.WriteRecord(dirs.State, record); err != nil {
		return fmt.Errorf("write the enrollment record: %w", err)
	}
	return nil
}

// profileDigest is the digest of the profile block, empty for core only.
func profileDigest(f policy.File) string {
	if f.Profile == nil {
		return ""
	}
	return f.Profile.Digest
}

// explain is the explain seam: every value of the policy in force with its
// source, rendered.
func (b binding) explain(ctx context.Context, root string, now time.Time) (string, error) {
	l, err := b.loaded(ctx, root, now)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	if err := policy.WriteExplain(&out, policy.Explain(l)); err != nil {
		return "", fmt.Errorf("render the explanation: %w", err)
	}
	return out.String(), nil
}

// render is the render seam: the report document of a run, as text or JSON.
func (b binding) render(w io.Writer, in model.Input, run *engine.Run, d engine.Decision, asJSON bool) error {
	id := report.Identity{
		KitVersion:   b.info.Version,
		PolicyDigest: in.Policy.Digest,
		Ruleset:      report.RulesetDigest(checks.Catalog()),
	}
	if s, ok := in.Snapshot.(identified); ok {
		id.Snapshot = s.Identity()
	}
	doc := report.New(run, d, id)
	write := report.WriteText
	if asJSON {
		write = report.WriteJSON
	}
	if err := write(w, doc); err != nil {
		return fmt.Errorf("write the report: %w", err)
	}
	return nil
}
