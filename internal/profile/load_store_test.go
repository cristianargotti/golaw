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

// stored copies the fixture bundle into the store directory of its pinned
// version under configDir and answers that directory.
func stored(t *testing.T, configDir string) string {
	t.Helper()
	dir := filepath.Dir(policy.StoredStampPath(configDir, profiletest.Name, profiletest.Version))
	if err := os.MkdirAll(filepath.Dir(dir), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(dir, os.DirFS(profiletest.Dir(t))); err != nil {
		t.Fatal(err)
	}
	return dir
}

// newerCurrent writes a current bundle of the fixture name at another
// version, the state after a newer install made itself current.
func newerCurrent(t *testing.T, configDir string) {
	t.Helper()
	dir := profile.Dir(configDir, profiletest.Name)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	stamp := "name: fixture\nversion: 1.3.0\ndate: 2026-10-21\nsource: " + profiletest.Source +
		"\ndigest: sha256:" + strings.Repeat("0", 64) + "\n"
	if err := os.WriteFile(filepath.Join(dir, policy.StampFile), []byte(stamp), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestLoadReadsTheStoredCopyOfThePinnedVersion pins contract D3 through the
// loader: a repository pinned at a version the store holds loads that copy
// whether the current bundle is newer or absent.
func TestLoadReadsTheStoredCopyOfThePinnedVersion(t *testing.T) {
	t.Parallel()
	for name, current := range map[string]bool{"beside a newer current bundle": true, "without a current bundle": false} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			config := t.TempDir()
			if current {
				newerCurrent(t, config)
			}
			stored(t, config)
			d, err := profile.Load(config, profiletest.Ref(t))
			if err != nil {
				t.Fatal(err)
			}
			if d.Version != profiletest.Version || d.Digest != profiletest.Ref(t).Digest || !d.Section(model.SectionDockerfile) {
				t.Errorf("Load = %+v, want the stored fixture data", d)
			}
		})
	}
}

// TestLoadRefusesATamperedStoredCopy pins D8 over the store: files changed
// under the stored stamp are ErrDigest naming both digests and the command.
func TestLoadRefusesATamperedStoredCopy(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	newerCurrent(t, config)
	dir := stored(t, config)
	if err := os.WriteFile(filepath.Join(dir, profile.RulesFile), []byte("golaw_profile: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := profile.Load(config, profiletest.Ref(t))
	want := "differs from its stamp " + profiletest.Ref(t).Digest + ": run golaw profile install " + profiletest.Source + "@" + profiletest.Version
	if !errors.Is(err, profile.ErrDigest) || !errors.Is(err, model.ErrPolicy) || !strings.HasSuffix(err.Error(), want) {
		t.Errorf("Load = %v, want ErrDigest ending with %q", err, want)
	}
}

// TestLoadNamesTheInstalledVersionThatDiffers pins the text of a pin the
// current bundle does not hold and the store does not either.
func TestLoadNamesTheInstalledVersionThatDiffers(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	newerCurrent(t, config)
	_, err := profile.Load(config, profiletest.Ref(t))
	want := "profile fixture: version is 1.2.3 in golaw.yml and 1.3.0 in the installed bundle: run golaw profile install " +
		profiletest.Source + "@" + profiletest.Version
	if !errors.Is(err, model.ErrPolicy) || !strings.HasSuffix(err.Error(), want) {
		t.Errorf("Load = %v, want it to end with %q", err, want)
	}
}
