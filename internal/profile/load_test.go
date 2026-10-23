package profile_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

func TestLoadAnswersTheDataOfThePinnedBundle(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	profiletest.Install(t, config)
	d, err := profile.Load(config, profiletest.Ref(t))
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != profiletest.Name || d.Digest != profiletest.Ref(t).Digest || !d.Section(model.SectionTerms) {
		t.Errorf("Load = %+v", d)
	}
	var loader policy.BundleLoader = profile.Load
	if _, err := loader(config, profiletest.Ref(t)); err != nil {
		t.Errorf("Load is a BundleLoader: %v", err)
	}
}

func TestLoadNamesTheInstallCommandForAMissingBundle(t *testing.T) {
	t.Parallel()
	_, err := profile.Load(t.TempDir(), profiletest.Ref(t))
	want := "profile fixture is not installed: run golaw profile install " + profiletest.Source + "@" + profiletest.Version
	if !errors.Is(err, model.ErrPolicy) || !strings.Contains(err.Error(), want) {
		t.Errorf("Load = %v, want the command %q", err, want)
	}
}

func TestLoadRefusesAMismatchedOrTamperedBundle(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	profiletest.Install(t, config)
	other := profiletest.Ref(t)
	other.Digest = "sha256:" + strings.Repeat("0", 64)
	if _, err := profile.Load(config, other); !errors.Is(err, model.ErrPolicy) || !strings.Contains(err.Error(), "digest is") {
		t.Errorf("a pin of another digest never falls back: %v", err)
	}
	tampered := t.TempDir()
	profiletest.Tampered(t, tampered)
	if _, err := profile.Load(tampered, profiletest.Ref(t)); !errors.Is(err, profile.ErrDigest) {
		t.Errorf("files changed under the stamp: %v", err)
	}
}

func TestLayoutOfTheConfigDirectory(t *testing.T) {
	t.Parallel()
	config := filepath.Join("home", "config")
	if got, want := profile.Dir(config, "fixture"), filepath.Dir(policy.StampPath(config, "fixture")); got != want {
		t.Errorf("Dir = %s, want %s", got, want)
	}
}
