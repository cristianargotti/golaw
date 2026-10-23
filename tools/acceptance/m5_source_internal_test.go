package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
)

func m5FixtureStamp(t *testing.T) policy.Stamp {
	t.Helper()
	stamp, err := policy.LoadStamp(filepath.Join(testRoot(t), filepath.FromSlash(m5BundleSource), policy.StampFile))
	if err != nil {
		t.Fatal(err)
	}
	return stamp
}

func TestM5SourceHoldsTwoVerifiedBundles(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	s.execute = runCommand
	bare, err := s.m5Source(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	fixture := m5FixtureStamp(t)
	for _, tc := range []struct{ tag, version string }{{m5Tag, m5Version}, {m5NewerTag, m5NewerVersion}} {
		dir := filepath.Join(t.TempDir(), tc.tag)
		got := runCommand(t.Context(), command{program: s.git, args: []string{"clone", "-q", "--branch", tc.tag, bare, dir}, dir: s.root, env: s.env})
		if err := success(got); err != nil {
			t.Fatalf("%s: %v", tc.tag, err)
		}
		bundle, err := profile.Read(dir)
		if err != nil {
			t.Fatalf("%s: %v", tc.tag, err)
		}
		if bundle.Stamp.Version != tc.version || bundle.Stamp.Source != m5Canonical || (tc.tag == m5Tag) != (bundle.Stamp.Digest == fixture.Digest) {
			t.Fatalf("%s stamp = %+v, fixture digest %s", tc.tag, bundle.Stamp, fixture.Digest)
		}
	}
}

func TestM5StampNamesTheDigestOfTheOtherFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := m5CopyBundle(testRoot(t), dir); err != nil {
		t.Fatal(err)
	}
	ref, err := m5Stamp(dir, m5Version, m5Date)
	if err != nil {
		t.Fatal(err)
	}
	if ref.Digest != m5FixtureStamp(t).Digest || ref.Name != m5Name || ref.Source != m5Canonical {
		t.Fatalf("the restamped fixture = %+v, want the fixture digest", ref)
	}
	if err = m5NewerBundle(dir); err != nil {
		t.Fatal(err)
	}
	newer, err := profile.Read(dir)
	if err != nil || newer.Stamp.Version != m5NewerVersion || newer.Stamp.Digest == ref.Digest {
		t.Fatalf("the newer bundle = %+v, %v", newer, err)
	}
	absent := filepath.Join(dir, "absent")
	if _, err := m5Stamp(absent, m5Version, m5Date); err == nil {
		t.Fatal("a missing bundle was stamped")
	}
	if err := m5NewerBundle(absent); err == nil {
		t.Fatal("a missing bundle was made newer")
	}
	if err := m5CopyBundle(t.TempDir(), filepath.Join(t.TempDir(), "copy")); err == nil {
		t.Fatal("a module without the fixture bundle was copied")
	}
	if _, err := os.Stat(filepath.Join(dir, policy.StampFile)); errors.Is(err, os.ErrNotExist) {
		t.Fatal("the stamp is absent")
	}
}
