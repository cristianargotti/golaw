package profile_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

// lifecycleDirs are the state and config directories of one test.
func lifecycleDirs(t *testing.T) policy.Dirs {
	t.Helper()
	return policy.Dirs{State: t.TempDir(), Config: t.TempDir()}
}

// installAt installs a source at a tag and fails the test on an error.
func installAt(t *testing.T, dirs policy.Dirs, url, tag string, at time.Time) (profile.Installed, bool) {
	t.Helper()
	src, err := profile.ParseSource(url + "@" + tag)
	if err != nil {
		t.Fatal(err)
	}
	installed, already, err := profile.Install(t.Context(), dirs, src, gittest.Isolated(), at)
	if err != nil {
		t.Fatal(err)
	}
	return installed, already
}

// recordOf reads the record of the fixture name, which must exist.
func recordOf(t *testing.T, dirs policy.Dirs) profile.Record {
	t.Helper()
	r, found, err := profile.ReadRecord(dirs.State, profiletest.Name)
	if err != nil || !found {
		t.Fatalf("ReadRecord = %+v, %t, %v", r, found, err)
	}
	return r
}

func TestInstallMakesTheBundleCurrentAndStoresIt(t *testing.T) {
	t.Parallel()
	dirs := lifecycleDirs(t)
	url, tag := profiletest.BareSource(t)
	installed, already := installAt(t, dirs, url, tag, now())
	ref := profiletest.Ref(t)
	want := profile.Installed{
		Name: ref.Name, Version: ref.Version, Digest: ref.Digest, Source: profiletest.Source,
		URL: url, Tag: tag, InstalledAt: now(),
	}
	if already || installed != want {
		t.Fatalf("Install = %+v, %t, want %+v", installed, already, want)
	}
	if _, err := policy.VerifyProfile(dirs.Config, ref); err != nil {
		t.Fatalf("the current bundle verifies against the pin: %v", err)
	}
	if _, err := profile.Read(filepath.Dir(policy.StoredStampPath(dirs.Config, ref.Name, ref.Version))); err != nil {
		t.Fatalf("the stored copy reads: %v", err)
	}
	if r := recordOf(t, dirs); r.Current != ref.Version || len(r.Versions) != 1 || r.Versions[0] != want || !r.Updated.Equal(now()) {
		t.Fatalf("record = %+v", r)
	}
	if left := leftovers(t, dirs.Config); !slices.Equal(left, []string{"fixture", "fixture@1.2.3"}) {
		t.Fatalf("the profiles directory holds %v", left)
	}
}

func TestInstallTwiceWritesNothing(t *testing.T) {
	t.Parallel()
	dirs := lifecycleDirs(t)
	url, tag := profiletest.BareSource(t)
	first, _ := installAt(t, dirs, url, tag, now())
	before, err := os.ReadFile(profile.RecordPath(dirs.State, profiletest.Name))
	if err != nil {
		t.Fatal(err)
	}
	again, already := installAt(t, dirs, url, tag, now().Add(time.Hour))
	after, err := os.ReadFile(profile.RecordPath(dirs.State, profiletest.Name))
	if err != nil || !already || again != first || !bytes.Equal(after, before) {
		t.Fatalf("a second install = %+v, %t, record changed %t, %v", again, already, !bytes.Equal(after, before), err)
	}
}

func TestInstallRepairsADamagedCurrentBundle(t *testing.T) {
	t.Parallel()
	dirs := lifecycleDirs(t)
	url, tag := profiletest.BareSource(t)
	installAt(t, dirs, url, tag, now())
	profiletest.Tampered(t, dirs.Config)
	later := now().Add(time.Hour)
	installed, already := installAt(t, dirs, url, tag, later)
	if already || !installed.InstalledAt.Equal(later) {
		t.Fatalf("a repair = %+v, %t", installed, already)
	}
	if _, err := profile.Read(profile.Dir(dirs.Config, profiletest.Name)); err != nil {
		t.Fatalf("the current bundle reads again: %v", err)
	}
}

func TestInstallKeepsEveryVersionInTheStore(t *testing.T) {
	t.Parallel()
	dirs := lifecycleDirs(t)
	url := bundleSource(t, release{version: "1.2.3"}, note("1.3.0", "the second version"))
	installAt(t, dirs, url, "v1.2.3", now())
	newer, _ := installAt(t, dirs, url, "v1.3.0", now().Add(time.Hour))
	r := recordOf(t, dirs)
	if r.Current != "1.3.0" || len(r.Versions) != 2 || r.Versions[1] != newer {
		t.Fatalf("record = %+v", r)
	}
	located, err := policy.LocateProfile(dirs.Config, profiletest.Ref(t))
	if err != nil || !located.Stored || located.Stamp.Version != "1.2.3" {
		t.Fatalf("a repository pinned at 1.2.3 still finds its bundle: %+v, %v", located, err)
	}
}

func TestInstallRefusesWhatItCannotTrust(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		source func(t *testing.T) (string, string)
		want   string
		is     error
	}{
		"a stamp that is not the tag": {func(t *testing.T) (string, string) {
			t.Helper()
			return bundleSource(t, release{version: "1.2.3", tag: "v2.0.0"}), "v2.0.0"
		}, "carries version 1.2.3 in its stamp, not 2.0.0", profile.ErrBundle},
		"an unknown key in rules.yml": {func(t *testing.T) (string, string) {
			t.Helper()
			return bundleSource(t, release{version: "1.2.3", files: map[string]string{"rules.yml": "golaw_profile: 1\nextra: 1\n"}}), "v1.2.3"
		}, "extra", profile.ErrBundle},
		"a stamp over other files": {func(t *testing.T) (string, string) {
			t.Helper()
			return bundleSource(t, release{version: "1.2.3", digest: profiletest.Ref(t).Digest, files: map[string]string{"ONBOARDING.md": "changed\n"}}), "v1.2.3"
		}, "differs from its stamp", profile.ErrDigest},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dirs := lifecycleDirs(t)
			url, tag := tc.source(t)
			src, err := profile.ParseSource(url + "@" + tag)
			if err != nil {
				t.Fatal(err)
			}
			_, _, err = profile.Install(t.Context(), dirs, src, gittest.Isolated(), now())
			if !errors.Is(err, tc.is) || !errors.Is(err, model.ErrPolicy) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Install = %v, want %q", err, tc.want)
			}
			if left := leftovers(t, dirs.Config); len(left) != 0 {
				t.Fatalf("a refused install left %v", left)
			}
			if _, found, err := profile.ReadRecord(dirs.State, profiletest.Name); err != nil || found {
				t.Fatalf("a refused install wrote a record: %t, %v", found, err)
			}
		})
	}
}

// The near miss of the tag rule: a tag without the v names the same version.
func TestInstallTakesATagWithoutTheV(t *testing.T) {
	t.Parallel()
	dirs := lifecycleDirs(t)
	url := bundleSource(t, release{version: "1.2.3", tag: "1.2.3"})
	if installed, _ := installAt(t, dirs, url, "1.2.3", now()); installed.Version != "1.2.3" || installed.Tag != "1.2.3" {
		t.Fatalf("Install = %+v", installed)
	}
}

// TestInstallTakesTheVFormOfABareTag is the other direction of the tag
// forms: a person who types v1.2.3 installs a source tagged 1.2.3.
func TestInstallTakesTheVFormOfABareTag(t *testing.T) {
	t.Parallel()
	dirs := lifecycleDirs(t)
	url := bundleSource(t, release{version: "1.2.3", tag: "1.2.3"})
	if installed, _ := installAt(t, dirs, url, "v1.2.3", now()); installed.Version != "1.2.3" || installed.Tag != "v1.2.3" {
		t.Fatalf("Install = %+v", installed)
	}
}

// TestInstallTakesARelativeSourcePath pins a source given as a path
// relative to where the person stands (golaw profile install
// ../bundle@1.2.3): the clone runs in the profiles directory, so the path
// is made absolute first, and the record keeps that absolute path for a
// refresh run from any other directory.
//
//nolint:paralleltest // the relative path is read against the working directory, which t.Chdir sets
func TestInstallTakesARelativeSourcePath(t *testing.T) {
	dirs := lifecycleDirs(t)
	url, tag := profiletest.BareSource(t)
	given, err := os.Stat(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Dir(url))
	installed, _ := installAt(t, dirs, filepath.Base(url), tag, now())
	kept, err := os.Stat(installed.URL)
	if err != nil || !filepath.IsAbs(installed.URL) || !os.SameFile(given, kept) || recordOf(t, dirs).Versions[0].URL != installed.URL {
		t.Fatalf("the record keeps the source as an absolute path to the same repository: %+v, %v", installed, err)
	}
}

// movedTagDirs installs the fixture, and drops its record when asked, so
// only the stored copy remembers the digest of the version.
func movedTagDirs(t *testing.T, dropRecord bool) policy.Dirs {
	t.Helper()
	dirs := lifecycleDirs(t)
	url, tag := profiletest.BareSource(t)
	installAt(t, dirs, url, tag, now())
	if dropRecord {
		if err := os.Remove(profile.RecordPath(dirs.State, profiletest.Name)); err != nil {
			t.Fatal(err)
		}
	}
	return dirs
}

func TestInstallRefusesAMovedTag(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{"named by the record": false, "named by the store alone": true}
	for name, dropRecord := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dirs := movedTagDirs(t, dropRecord)
			moved, err := profile.ParseSource(bundleSource(t, note("1.2.3", "the tag moved")) + "@v1.2.3")
			if err != nil {
				t.Fatal(err)
			}
			_, _, err = profile.Install(t.Context(), dirs, moved, gittest.Isolated(), now())
			if !errors.Is(err, profile.ErrBundle) || !strings.Contains(err.Error(), "a published version never changes") ||
				!strings.Contains(err.Error(), profiletest.Ref(t).Digest) {
				t.Fatalf("a moved tag = %v", err)
			}
			if _, err := policy.VerifyProfile(dirs.Config, profiletest.Ref(t)); err != nil {
				t.Fatalf("the installed bundle is untouched: %v", err)
			}
		})
	}
}
