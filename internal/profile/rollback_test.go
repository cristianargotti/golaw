package profile_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

// twoVersions installs 1.2.3, the fixture, then 1.3.0 from one source.
func twoVersions(t *testing.T, dirs policy.Dirs) string {
	t.Helper()
	url := bundleSource(t, release{version: "1.2.3"}, note("1.3.0", "the second version"))
	installAt(t, dirs, url, "v1.2.3", now())
	installAt(t, dirs, url, "v1.3.0", now().Add(time.Hour))
	return url
}

// currentVersion is the version the current bundle's stamp carries.
func currentVersion(t *testing.T, dirs policy.Dirs) string {
	t.Helper()
	stamp, err := policy.LoadStamp(policy.StampPath(dirs.Config, profiletest.Name))
	if err != nil {
		t.Fatal(err)
	}
	return stamp.Version
}

func TestRollbackReturnsToThePreviousVersionAndBack(t *testing.T) {
	t.Parallel()
	dirs := lifecycleDirs(t)
	twoVersions(t, dirs)
	back := now().Add(2 * time.Hour)
	previous, err := profile.Rollback(dirs, profiletest.Name, back)
	if err != nil || previous.Version != "1.2.3" || currentVersion(t, dirs) != "1.2.3" {
		t.Fatalf("Rollback = %+v, %v, current %s", previous, err, currentVersion(t, dirs))
	}
	located, err := policy.LocateProfile(dirs.Config, profiletest.Ref(t))
	if err != nil || located.Stored {
		t.Fatalf("a repository pinned at 1.2.3 reads the current bundle again: %+v, %v", located, err)
	}
	r := recordOf(t, dirs)
	if r.Current != "1.2.3" || !r.Updated.Equal(back) || !slices.Equal([]string{r.Versions[0].Version, r.Versions[1].Version}, []string{"1.3.0", "1.2.3"}) {
		t.Fatalf("record = %+v", r)
	}
	if again, err := profile.Rollback(dirs, profiletest.Name, back.Add(time.Hour)); err != nil || again.Version != "1.3.0" || currentVersion(t, dirs) != "1.3.0" {
		t.Fatalf("a second rollback = %+v, %v", again, err)
	}
}

func TestRollbackRefusesWhatItCannotRollBack(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		name  string
		state func(t *testing.T, dirs policy.Dirs)
		want  string
		is    error
	}{
		"a name that is not one": {"Fixture", func(*testing.T, policy.Dirs) {}, `"Fixture" is not a profile name`, model.ErrUsage},
		"no record":              {profiletest.Name, func(*testing.T, policy.Dirs) {}, "has no install record", profile.ErrBundle},
		"one version": {profiletest.Name, func(t *testing.T, dirs policy.Dirs) {
			t.Helper()
			url, tag := profiletest.BareSource(t)
			installAt(t, dirs, url, tag, now())
		}, "no earlier version to roll back to: 1.2.3 is the only one installed", profile.ErrBundle},
		"a stored copy removed": {profiletest.Name, func(t *testing.T, dirs policy.Dirs) {
			t.Helper()
			twoVersions(t, dirs)
			if err := os.RemoveAll(filepath.Dir(policy.StoredStampPath(dirs.Config, profiletest.Name, "1.2.3"))); err != nil {
				t.Fatal(err)
			}
		}, "the stored copy cannot be read", profile.ErrBundle},
		"a record that names another digest": {profiletest.Name, func(t *testing.T, dirs policy.Dirs) {
			t.Helper()
			twoVersions(t, dirs)
			r := recordOf(t, dirs)
			r.Versions[0].Digest = "sha256:" + strings.Repeat("0", 64)
			if err := profile.WriteRecord(dirs.State, r); err != nil {
				t.Fatal(err)
			}
		}, "and the record sha256:" + strings.Repeat("0", 64), profile.ErrBundle},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dirs := lifecycleDirs(t)
			tc.state(t, dirs)
			_, err := profile.Rollback(dirs, tc.name, now())
			if !errors.Is(err, tc.is) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Rollback = %v, want %q", err, tc.want)
			}
		})
	}
}

// A refused rollback names the install command of the version it wanted.
func TestRollbackNamesTheInstallOfTheVersionItWanted(t *testing.T) {
	t.Parallel()
	dirs := lifecycleDirs(t)
	url := twoVersions(t, dirs)
	if err := os.RemoveAll(filepath.Dir(policy.StoredStampPath(dirs.Config, profiletest.Name, "1.2.3"))); err != nil {
		t.Fatal(err)
	}
	_, err := profile.Rollback(dirs, profiletest.Name, now())
	if err == nil || !strings.Contains(err.Error(), "run golaw profile install "+url+"@v1.2.3") || currentVersion(t, dirs) != "1.3.0" {
		t.Fatalf("Rollback = %v, current %s", err, currentVersion(t, dirs))
	}
}
