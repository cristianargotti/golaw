package profile_test

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

// recordFrom writes a record of the fixture name whose current version
// came from url, the state a refresh starts from.
func recordFrom(t *testing.T, dirs policy.Dirs, url string) {
	t.Helper()
	ref := profiletest.Ref(t)
	current := profile.Installed{
		Name: ref.Name, Version: ref.Version, Digest: ref.Digest, Source: ref.Source,
		URL: url, Tag: profiletest.Tag, InstalledAt: now(),
	}
	record := profile.Record{Name: ref.Name, Current: ref.Version, Versions: []profile.Installed{current}, Updated: now()}
	if err := profile.WriteRecord(dirs.State, record); err != nil {
		t.Fatal(err)
	}
}

func TestRefreshRepairsTheCurrentBundle(t *testing.T) {
	t.Parallel()
	dirs := lifecycleDirs(t)
	url, tag := profiletest.BareSource(t)
	installAt(t, dirs, url, tag, now())
	profiletest.Tampered(t, dirs.Config)
	later := now().Add(time.Hour)
	installed, err := profile.Refresh(t.Context(), dirs, profiletest.Name, gittest.Isolated(), later)
	if err != nil || installed.URL != url || installed.Tag != tag || !installed.InstalledAt.Equal(later) {
		t.Fatalf("Refresh = %+v, %v", installed, err)
	}
	if _, err := policy.VerifyProfile(dirs.Config, profiletest.Ref(t)); err != nil {
		t.Fatalf("the current bundle verifies again: %v", err)
	}
	if _, err := profile.Read(profile.Dir(dirs.Config, profiletest.Name)); err != nil {
		t.Fatalf("the current bundle reads again: %v", err)
	}
}

func TestRefreshOfASoundBundleWritesNothing(t *testing.T) {
	t.Parallel()
	dirs := lifecycleDirs(t)
	url, tag := profiletest.BareSource(t)
	first, _ := installAt(t, dirs, url, tag, now())
	before, err := os.ReadFile(profile.RecordPath(dirs.State, profiletest.Name))
	if err != nil {
		t.Fatal(err)
	}
	installed, err := profile.Refresh(t.Context(), dirs, profiletest.Name, gittest.Isolated(), now().Add(time.Hour))
	after, readErr := os.ReadFile(profile.RecordPath(dirs.State, profiletest.Name))
	if err != nil || readErr != nil || installed != first || !bytes.Equal(before, after) {
		t.Fatalf("Refresh = %+v, %v, %v", installed, err, readErr)
	}
}

func TestRefreshRefusesWhatItCannotRefresh(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		name  string
		state func(t *testing.T, dirs policy.Dirs)
		want  string
		is    error
	}{
		"a name that is not one": {"../fixture", func(*testing.T, policy.Dirs) {}, `"../fixture" is not a profile name`, model.ErrUsage},
		"no record":              {profiletest.Name, func(*testing.T, policy.Dirs) {}, "profile fixture has no install record: run golaw profile install <git url>@<tag>", profile.ErrBundle},
		"a source that now holds another profile": {profiletest.Name, func(t *testing.T, dirs policy.Dirs) {
			t.Helper()
			recordFrom(t, dirs, bundleSource(t, release{name: "other", version: "1.2.3"}))
		}, "now holds the profile other", profile.ErrBundle},
		"a tag that moved": {profiletest.Name, func(t *testing.T, dirs policy.Dirs) {
			t.Helper()
			url, tag := profiletest.BareSource(t)
			installAt(t, dirs, url, tag, now())
			recordFrom(t, dirs, bundleSource(t, note("1.2.3", "the tag moved")))
		}, "a published version never changes", profile.ErrBundle},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dirs := lifecycleDirs(t)
			tc.state(t, dirs)
			_, err := profile.Refresh(t.Context(), dirs, tc.name, gittest.Isolated(), now())
			if !errors.Is(err, tc.is) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Refresh = %v, want %q", err, tc.want)
			}
		})
	}
}
