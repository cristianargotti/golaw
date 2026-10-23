package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

// profiledRepository is a committed repository whose golaw.yml pins the
// fixture bundle, the policy of a repository enrolled under a profile.
func profiledRepository(t *testing.T) string {
	t.Helper()
	ref := profiletest.Ref(t)
	in, err := bootstrap.Inputs{
		KitVersion: testVersion, Language: model.DefaultLanguage, Profile: &ref,
		Layout: bootstrap.Layout{Main: bootstrap.MainRoot, Decisions: "docs/adr", Go: true},
	}.Normalize()
	if err != nil {
		t.Fatal(err)
	}
	data, err := bootstrap.Render(in)
	if err != nil {
		t.Fatal(err)
	}
	root := repository(t, false)
	if err := os.WriteFile(filepath.Join(root, policy.FileName), data, 0o600); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "build: pin the profile")
	return root
}

func TestBindingLoadsTheDataOfThePinnedBundle(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	dirs, err := b.dirs()
	if err != nil {
		t.Fatal(err)
	}
	profiletest.Install(t, dirs.Config)
	root := profiledRepository(t)
	l, err := b.loaded(t.Context(), root, now())
	if err != nil {
		t.Fatal(err)
	}
	if l.Data == nil || l.Data.Name != profiletest.Name || l.Policy.ProfileData != l.Data || l.Stamp == nil {
		t.Fatalf("loaded = %+v", l)
	}
	if !slices.Contains(l.Policy.Terms.Tools, "legacyci") || l.Policy.Profile != profiletest.Name {
		t.Fatalf("policy = %+v", l.Policy)
	}
	p, err := b.load(t.Context(), root, now())
	if err != nil || p.ProfileData == nil {
		t.Fatalf("load = %+v, %v", p, err)
	}
}

func TestBindingRefusesATamperedBundleNamingTheInstallCommand(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	dirs, err := b.dirs()
	if err != nil {
		t.Fatal(err)
	}
	profiletest.Tampered(t, dirs.Config)
	root := profiledRepository(t)
	_, err = b.loaded(t.Context(), root, now())
	if !errors.Is(err, profile.ErrDigest) || !errors.Is(err, model.ErrPolicy) {
		t.Fatalf("a tampered bundle = %v", err)
	}
	if !strings.Contains(err.Error(), "run golaw profile install "+profiletest.Source+"@"+profiletest.Version) {
		t.Fatalf("the command is not named: %v", err)
	}
}

func TestBindingNamesTheInstallCommandWithoutABundle(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	root := profiledRepository(t)
	_, err := b.load(t.Context(), root, now())
	want := "profile " + profiletest.Name + " is not installed: run golaw profile install " + profiletest.Source + "@" + profiletest.Version
	if !errors.Is(err, model.ErrPolicy) || !strings.Contains(err.Error(), want) {
		t.Fatalf("a missing bundle = %v", err)
	}
}
