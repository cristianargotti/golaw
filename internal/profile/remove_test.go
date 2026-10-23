package profile_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

func TestRemoveDeletesEveryCopyAndTheRecord(t *testing.T) {
	t.Parallel()
	dirs := lifecycleDirs(t)
	twoVersions(t, dirs)
	if err := profile.Remove(dirs, profiletest.Name); err != nil {
		t.Fatal(err)
	}
	if left := leftovers(t, dirs.Config); len(left) != 0 {
		t.Fatalf("remove left %v", left)
	}
	if _, found, err := profile.ReadRecord(dirs.State, profiletest.Name); err != nil || found {
		t.Fatalf("the record is gone: %t, %v", found, err)
	}
	_, err := policy.VerifyProfile(dirs.Config, profiletest.Ref(t))
	if want := "profile fixture is not installed: run golaw profile install " + profiletest.Source + "@" + profiletest.Version; err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("a pinned repository meets the install command: %v", err)
	}
}

func TestRemoveTakesWhatARecordDoesNotList(t *testing.T) {
	t.Parallel()
	cases := map[string]func(t *testing.T, dirs policy.Dirs){
		"a bundle placed without a record": func(t *testing.T, dirs policy.Dirs) {
			t.Helper()
			profiletest.Install(t, dirs.Config)
		},
		"a damaged record": func(t *testing.T, dirs policy.Dirs) {
			t.Helper()
			url, tag := profiletest.BareSource(t)
			installAt(t, dirs, url, tag, now())
			if err := os.WriteFile(profile.RecordPath(dirs.State, profiletest.Name), []byte("{"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, state := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dirs := lifecycleDirs(t)
			state(t, dirs)
			if err := profile.Remove(dirs, profiletest.Name); err != nil {
				t.Fatal(err)
			}
			if left := leftovers(t, dirs.Config); len(left) != 0 {
				t.Fatalf("remove left %v", left)
			}
		})
	}
}

// The near miss of the name match: a name the removed one prefixes keeps
// its current bundle and its stored copies.
func TestRemoveLeavesANameItPrefixesAlone(t *testing.T) {
	t.Parallel()
	dirs := lifecycleDirs(t)
	profiletest.Install(t, dirs.Config)
	root := filepath.Dir(profile.Dir(dirs.Config, profiletest.Name))
	for _, other := range []string{"fixture-two", "fixture-two@1.0.0"} {
		if err := os.MkdirAll(filepath.Join(root, other), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := profile.Remove(dirs, profiletest.Name); err != nil {
		t.Fatal(err)
	}
	if left := leftovers(t, dirs.Config); !slices.Equal(left, []string{"fixture-two", "fixture-two@1.0.0"}) {
		t.Fatalf("remove took another name: %v", left)
	}
}

func TestRemoveRefusesWhatIsNotInstalled(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		name string
		want string
		is   error
	}{
		"nothing installed":      {profiletest.Name, "profile fixture is not installed", profile.ErrBundle},
		"a name that is not one": {"../state", `"../state" is not a profile name`, model.ErrUsage},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := profile.Remove(lifecycleDirs(t), tc.name); !errors.Is(err, tc.is) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Remove = %v, want %q", err, tc.want)
			}
		})
	}
}

// notDirIsAbsent holds on a system where Go reads a regular file in the
// place of a directory as nothing there: Windows answers a file opened as
// a directory, or a path under one, with ERROR_PATH_NOT_FOUND, the errno
// Go names ENOTDIR there, which errors.Is matches to fs.ErrNotExist,
// where Unix answers ENOTDIR.
var notDirIsAbsent = errors.Is(syscall.ENOTDIR, fs.ErrNotExist)

// TestRemoveNamesAProfilesDirectoryItCannotRead names the profiles
// directory a file stands in for, or, where the system reads that file as
// nothing there, the name as not installed.
func TestRemoveNamesAProfilesDirectoryItCannotRead(t *testing.T) {
	t.Parallel()
	dirs := lifecycleDirs(t)
	root := filepath.Dir(profile.Dir(dirs.Config, profiletest.Name))
	if err := os.WriteFile(root, []byte("a file where the profiles directory goes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := "read "
	if notDirIsAbsent {
		want = "profile " + profiletest.Name + " is not installed"
	}
	if err := profile.Remove(dirs, profiletest.Name); !errors.Is(err, profile.ErrBundle) || !strings.Contains(err.Error(), want) {
		t.Fatalf("Remove over a file = %v, want %q", err, want)
	}
}
