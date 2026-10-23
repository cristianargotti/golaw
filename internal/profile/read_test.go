package profile_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

func TestReadTheFixtureBundle(t *testing.T) {
	t.Parallel()
	b, err := profile.Read(profiletest.Dir(t))
	if err != nil {
		t.Fatal(err)
	}
	ref := profiletest.Ref(t)
	if b.Stamp.Name != profiletest.Name || b.Stamp.Version != profiletest.Version || b.Stamp.Source != profiletest.Source {
		t.Errorf("stamp = %+v", b.Stamp)
	}
	if b.Data.Name != ref.Name || b.Data.Version != ref.Version || b.Data.Digest != ref.Digest || b.Dir != profiletest.Dir(t) {
		t.Errorf("the identity rides on the data: %+v", b.Data)
	}
	if !b.Data.Section(model.SectionAssertions) || !b.Data.Section(model.SectionDockerfile) {
		t.Errorf("the data is decoded: %+v", b.Data)
	}
}

func TestFixtureStampCarriesItsDigest(t *testing.T) {
	t.Parallel()
	dir := profiletest.Dir(t)
	got, err := profile.Digest(dir)
	if err != nil {
		t.Fatal(err)
	}
	s, err := policy.LoadStamp(filepath.Join(dir, policy.StampFile))
	if err != nil {
		t.Fatal(err)
	}
	if s.Digest != got {
		t.Fatalf("the fixture stamp pins %s and its files hash to %s: rewrite the digest line of profile.yml", s.Digest, got)
	}
}

func TestReadRefusesABundleWhoseFilesMoved(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	profiletest.Tampered(t, config)
	_, err := profile.Read(profile.Dir(config, profiletest.Name))
	if !errors.Is(err, profile.ErrDigest) || !errors.Is(err, model.ErrPolicy) || !strings.Contains(err.Error(), "differs from its stamp") {
		t.Errorf("a changed file is ErrDigest under ErrPolicy naming both digests: %v", err)
	}
}

func TestReadRefusesABundleWithoutRules(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	profiletest.Install(t, config)
	dir := profile.Dir(config, profiletest.Name)
	if err := os.Remove(filepath.Join(dir, profile.RulesFile)); err != nil {
		t.Fatal(err)
	}
	// The stamp still names the old digest, so the digest check answers first.
	if _, err := profile.Read(dir); !errors.Is(err, model.ErrPolicy) {
		t.Errorf("Read = %v", err)
	}
	if _, err := profile.Read(filepath.Join(t.TempDir(), "absent")); !errors.Is(err, model.ErrPolicy) {
		t.Errorf("an absent directory: %v", err)
	}
}

func TestReadRefusesAStampAndRulesThatDisagreeOnTheSchema(t *testing.T) {
	t.Parallel()
	dir := writeBundle(t, map[string]string{profile.RulesFile: "golaw_profile: 1\nterms: [Bad]\n"})
	digest, err := profile.Digest(dir)
	if err != nil {
		t.Fatal(err)
	}
	stamp := "name: fixture\nversion: 1.2.3\ndate: 2026-10-20\nsource: " + profiletest.Source + "\ndigest: " + digest + "\n"
	if err := os.WriteFile(filepath.Join(dir, policy.StampFile), []byte(stamp), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := profile.Read(dir); !errors.Is(err, profile.ErrBundle) || !strings.Contains(err.Error(), "terms") {
		t.Errorf("invalid data behind a true digest is ErrBundle: %v", err)
	}
}
