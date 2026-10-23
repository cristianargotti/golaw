package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// profileNamePattern is the grammar of a profile name: lower case, so the
// name stays one directory under the config directory.
var profileNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// installForm is the install command of a first pin, whose source only the
// person knows.
const installForm = "golaw profile install <git url>@<tag>"

// resolveProfile turns --profile into the pin init writes, once and before
// any file of the checkout is read: the stamp of the installed bundle of
// that name (D7), never the network. It is refused beside --update, which
// keeps the reviewed policy and moves the kit pin alone.
func (a *app) resolveProfile(ctx context.Context, path string, opts *initOptions) error {
	if opts.profile == "" {
		return nil
	}
	if opts.update {
		return fmt.Errorf("%w: --profile enrolls a pin, while init --update keeps the policy and moves the kit pin", model.ErrUsage)
	}
	ref, err := a.bind().profilePin(ctx, path, opts.profile)
	if err != nil {
		return err
	}
	opts.pin = &ref
	return nil
}

// profilePin is the reference init pins for an installed bundle: the stamp
// of the current bundle of the name under the config directory. A name
// without a bundle is a usage error naming the install command (A19): the
// one the pin at HEAD carries when HEAD already pins that name, otherwise
// the form, because the kit cannot know the source of a first pin; a stamp
// of another name is refused, so the pin never names a directory that
// holds a different bundle.
func (b binding) profilePin(ctx context.Context, dir, name string) (policy.ProfileRef, error) {
	if !profileNamePattern.MatchString(name) {
		return policy.ProfileRef{}, fmt.Errorf("%w: profile name %q is not a lower case name", model.ErrUsage, name)
	}
	dirs, err := b.dirs()
	if err != nil {
		return policy.ProfileRef{}, err
	}
	path := policy.StampPath(dirs.Config, name)
	s, err := policy.LoadStamp(path)
	if errors.Is(err, fs.ErrNotExist) {
		return policy.ProfileRef{}, fmt.Errorf("%w: profile %s is not installed: run %s", model.ErrUsage, name, installCommand(ctx, dir, name))
	}
	if err != nil {
		return policy.ProfileRef{}, fmt.Errorf("read the stamp of profile %s: %w", name, err)
	}
	if s.Name != name {
		return policy.ProfileRef{}, fmt.Errorf("%w: the bundle under %s is named %s, not %s", model.ErrUsage, filepath.Dir(path), s.Name, name)
	}
	return policy.ProfileRef{Name: s.Name, Source: s.Source, Version: s.Version, Digest: s.Digest}, nil
}

// installCommand is the command that installs the bundle of a name: the
// source and version golaw.yml pins at HEAD of the repository that holds
// dir, when HEAD pins that name, as every refusal of D8 names them.
func installCommand(ctx context.Context, dir, name string) string {
	root, err := bootstrap.Root(dir)
	if err != nil {
		return installForm
	}
	head, ok := headProfile(ctx, root)
	if !ok || head.Name != name {
		return installForm
	}
	return "golaw profile install " + head.Source + "@" + head.Version
}

// headProfile is the profile golaw.yml pins at HEAD, false for none.
func headProfile(ctx context.Context, root string) (policy.ProfileRef, bool) {
	data, found, err := policy.ReadHead(ctx, root)
	if err != nil || !found {
		return policy.ProfileRef{}, false
	}
	f, err := policy.Decode(data)
	if err != nil || f.Profile == nil {
		return policy.ProfileRef{}, false
	}
	return *f.Profile, true
}
