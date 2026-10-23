package profile_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

// fixtureBundle is the fixture read whole, as Install hands it to Store.
func fixtureBundle(t *testing.T) *profile.Bundle {
	t.Helper()
	b, err := profile.Read(profiletest.Dir(t))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// storedFixture is the stored copy of the fixture version.
func storedFixture(config string) string {
	return filepath.Dir(policy.StoredStampPath(config, profiletest.Name, profiletest.Version))
}

func TestStoreWritesOnceThenNothing(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	if stored, err := profile.Store(config, fixtureBundle(t)); err != nil || !stored {
		t.Fatalf("a first store = %t, %v", stored, err)
	}
	if stored, err := profile.Store(config, fixtureBundle(t)); err != nil || stored {
		t.Fatalf("a second store = %t, %v", stored, err)
	}
	for _, dir := range []string{profile.Dir(config, profiletest.Name), storedFixture(config)} {
		if b, err := profile.Read(dir); err != nil || b.Data.Digest != profiletest.Ref(t).Digest {
			t.Fatalf("%s reads as the fixture: %v", dir, err)
		}
	}
}

func TestStoreLeavesTheGitDirectoryOut(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	clone, err := profile.Fetch(t.Context(), config, fixtureSource(t), gittest.Isolated())
	if err != nil {
		t.Fatal(err)
	}
	b, err := profile.Read(clone)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := profile.Store(config, b); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{profile.Dir(config, profiletest.Name), storedFixture(config)} {
		if _, err := os.Stat(filepath.Join(dir, ".git")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s carries .git: %v", dir, err)
		}
	}
}

func TestStoreCopiesNestedFiles(t *testing.T) {
	t.Parallel()
	dirs := lifecycleDirs(t)
	url := bundleSource(t, release{version: "1.2.3", files: map[string]string{"docs/notes.md": "nested data\n"}})
	installAt(t, dirs, url, "v1.2.3", now())
	data, err := os.ReadFile(filepath.Join(profile.Dir(dirs.Config, profiletest.Name), "docs", "notes.md"))
	if err != nil || string(data) != "nested data\n" {
		t.Fatalf("the nested file = %q, %v", data, err)
	}
}

func TestStoreReplacesADamagedStoredCopy(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	if _, err := profile.Store(config, fixtureBundle(t)); err != nil {
		t.Fatal(err)
	}
	rules := filepath.Join(storedFixture(config), profile.RulesFile)
	if err := os.WriteFile(rules, []byte("golaw_profile: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if stored, err := profile.Store(config, fixtureBundle(t)); err != nil || !stored {
		t.Fatalf("a repair = %t, %v", stored, err)
	}
	if _, err := profile.Read(storedFixture(config)); err != nil {
		t.Fatalf("the stored copy reads again: %v", err)
	}
}

func TestStoreRefusesWhatItCannotCopy(t *testing.T) {
	t.Parallel()
	source := t.TempDir()
	for name, content := range fixtureFiles(t) {
		if err := os.WriteFile(filepath.Join(source, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(source, profile.RulesFile), filepath.Join(source, "link.yml")); err != nil {
		t.Skipf("symbolic links need a privilege on this system: %v", err)
	}
	b := fixtureBundle(t)
	b.Dir = source
	config := t.TempDir()
	if _, err := profile.Store(config, b); !errors.Is(err, profile.ErrBundle) || !strings.Contains(err.Error(), "link.yml is not a regular file") {
		t.Fatalf("a bundle with a link = %v", err)
	}
	if left := leftovers(t, config); len(left) != 0 {
		t.Fatalf("a refused copy left %v", left)
	}
}

func TestStoreNamesAConfigDirectoryItCannotWrite(t *testing.T) {
	t.Parallel()
	config := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(config, []byte("a file where the config directory goes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := profile.Store(config, fixtureBundle(t)); !errors.Is(err, profile.ErrBundle) || !strings.Contains(err.Error(), "create ") {
		t.Fatalf("Store over a file = %v", err)
	}
}

// The current bundle is placed after the stored copy, so a copy that
// fails there must leave the stored copy and the profiles directory as
// they were, the damaged current bundle included, for the next install.
func TestStoreNamesACurrentBundleItCannotPlace(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	if _, err := profile.Store(config, fixtureBundle(t)); err != nil {
		t.Fatal(err)
	}
	profiletest.Tampered(t, config)
	b := fixtureBundle(t)
	b.Dir = filepath.Join(t.TempDir(), "gone")
	if _, err := profile.Store(config, b); !errors.Is(err, profile.ErrBundle) || !strings.Contains(err.Error(), "gone") {
		t.Fatalf("a bundle whose files are gone = %v", err)
	}
	if left := leftovers(t, config); !slices.Equal(left, []string{"fixture", "fixture@1.2.3"}) {
		t.Fatalf("a failed placement left %v", left)
	}
	if _, err := profile.Read(storedFixture(config)); err != nil {
		t.Fatalf("the stored copy is untouched: %v", err)
	}
}
