package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
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
	exe     func() (string, error)
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

func discoverFailed(dir string, err error) error {
	return fmt.Errorf("discover %s: %w", dir, err)
}

// locate hands the policy package the source discovery, which the package
// boundaries of section 5.1 keep it from importing.
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

func (b binding) loaded(ctx context.Context, dir string, now time.Time) (*policy.Loaded, error) {
	return b.loadedFor(ctx, dir, now, false)
}

// loadedFor is the policy in force; a record writer reenrolls, so a
// committed tightening the record does not hold yet loads with the
// mismatch state instead of refusing (A3).
func (b binding) loadedFor(ctx context.Context, dir string, now time.Time, reenroll bool) (*policy.Loaded, error) {
	return b.loadedBy(ctx, policy.Input{Dir: dir, Now: now, Reenroll: reenroll, Discover: locate})
}

// loadedBy is the policy in force for the directory, the clock, the
// discoverer and the switches of an input, such as the cache of a hook
// process and the policy it takes before the first commit; the binding
// fills the directories, the pin and the schemas.
func (b binding) loadedBy(ctx context.Context, in policy.Input) (*policy.Loaded, error) {
	dirs, err := b.dirs()
	if err != nil {
		return nil, err
	}
	in.Dirs, in.Version, in.Schemas = dirs, b.pin(), schemas()
	l, err := policy.Load(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("load the policy of %s: %w", in.Dir, err)
	}
	return l, nil
}

// load is the load seam: the effective policy in force, refused on an
// enrolled checkout that no longer holds the file (A3).
func (b binding) load(ctx context.Context, root string, now time.Time) (*model.EffectivePolicy, error) {
	l, err := b.loaded(ctx, root, now)
	if err != nil {
		return nil, err
	}
	if err := enrolledCheckout(l); err != nil {
		return nil, err
	}
	return l.Policy, nil
}

// enrolledCheckout refuses an enrolled repository whose checkout no longer
// holds golaw.yml (A3). The policy in force comes from HEAD or from the
// bootstrap record, so neither shows a deletion that only the checkout
// holds; scan and the hook ask the checkout itself.
func enrolledCheckout(l *policy.Loaded) error {
	if l.Record == nil {
		return nil
	}
	present, err := inCheckout(l.Root)
	if err != nil {
		return err
	}
	if !present {
		return fmt.Errorf("%w: %s is gone from the checkout of %s (%s)", model.ErrEnrollment, policy.FileName, l.Root, model.EnrollmentMissing)
	}
	return nil
}

// inCheckout answers whether the checkout of root holds golaw.yml. Only an
// absent file is an absence: any other failure to inspect it is an error,
// so a denial never names the wrong cause.
func inCheckout(root string) (bool, error) {
	_, err := os.Lstat(filepath.Join(root, policy.FileName))
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	default:
		return false, fmt.Errorf("inspect the %s of %s: %w", policy.FileName, root, err)
	}
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
// with the bytes written as the bootstrap policy of an unborn branch. A
// record that already holds another policy moves only to a tightening of
// it, so init never enrolls the defaults over a reviewed policy.
func (b binding) enroll(root string, data []byte, p *model.EffectivePolicy, now time.Time) error {
	dirs, err := b.dirs()
	if err != nil {
		return err
	}
	f, err := policy.Decode(data)
	if err != nil {
		return fmt.Errorf("decode the written policy: %w", err)
	}
	existing, found, err := policy.ReadRecord(dirs.State, root)
	if err != nil {
		return fmt.Errorf("read the enrollment record of %s: %w", root, err)
	}
	if found && existing.PolicyDigest != p.Digest {
		if err := policy.CheckReenrollment(existing, f, schemas()); err != nil {
			return fmt.Errorf("enroll %s: %w", root, err)
		}
	}
	record := policy.Record{
		Root: root, PolicyDigest: p.Digest, ProfileDigest: profileDigest(f), KitVersion: p.KitVersion,
		Bootstrap: string(data), WrittenBy: policy.WrittenByInit, WrittenAt: now,
	}
	if found && existing.Root == record.Root && existing.PolicyDigest == record.PolicyDigest &&
		existing.ProfileDigest == record.ProfileDigest && existing.KitVersion == record.KitVersion &&
		existing.Bootstrap == record.Bootstrap && existing.WrittenBy == record.WrittenBy {
		return nil
	}
	if err := policy.WriteRecord(dirs.State, record); err != nil {
		return fmt.Errorf("write the enrollment record: %w", err)
	}
	return nil
}

// committed is the committed seam: whether HEAD holds the golaw.yml of a
// digest, the policy init enrolled; an unborn branch holds none.
func (b binding) committed(ctx context.Context, root, digest string) (bool, error) {
	repo, err := source.Discover(ctx, root)
	if err != nil {
		return false, discoverFailed(root, err)
	}
	if repo.Head == "" {
		return false, nil
	}
	data, found, err := policy.ReadHead(ctx, repo.Root)
	if err != nil {
		return false, fmt.Errorf("read the policy at HEAD of %s: %w", repo.Root, err)
	}
	return found && digestOf(data) == digest, nil
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
