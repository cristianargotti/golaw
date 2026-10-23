package profile_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/profile"
)

// installedAt is one installed version of the fixture name as a record keeps it.
func installedAt(version, digest string, at time.Time) profile.Installed {
	return profile.Installed{
		Name: "fixture", Version: version, Digest: digest, Source: "https://example.test/profiles/fixture",
		URL: "/srv/profiles/fixture.git", Tag: "v" + version, InstalledAt: at,
	}
}

func TestRecordPathIsOneFilePerNameUnderTheState(t *testing.T) {
	t.Parallel()
	state := t.TempDir()
	if got, want := profile.RecordPath(state, "fixture"), filepath.Join(state, "profiles", "fixture.json"); got != want {
		t.Fatalf("RecordPath = %s, want %s", got, want)
	}
}

func TestWriteRecordThenReadItBack(t *testing.T) {
	t.Parallel()
	state := t.TempDir()
	written := profile.Record{
		Name: "fixture", Current: "1.3.0", Updated: now(),
		Versions: []profile.Installed{installedAt("1.2.3", "sha256:aa", now()), installedAt("1.3.0", "sha256:bb", now())},
	}
	if err := profile.WriteRecord(state, written); err != nil {
		t.Fatal(err)
	}
	got, found, err := profile.ReadRecord(state, "fixture")
	written.Schema = 1
	if err != nil || !found || !reflect.DeepEqual(got, written) {
		t.Fatalf("ReadRecord = %+v, %t, %v, want %+v", got, found, err, written)
	}
	if runtime.GOOS == "windows" {
		return
	}
	for path, want := range map[string]os.FileMode{
		profile.RecordPath(state, "fixture"):               0o600,
		filepath.Dir(profile.RecordPath(state, "fixture")): 0o700,
	} {
		if info, err := os.Stat(path); err != nil || info.Mode().Perm() != want {
			t.Errorf("%s: mode %v, %v, want %v", path, info.Mode().Perm(), err, want)
		}
	}
}

func TestReadRecordWithoutOneIsNotFound(t *testing.T) {
	t.Parallel()
	got, found, err := profile.ReadRecord(t.TempDir(), "fixture")
	if err != nil || found || got.Name != "" {
		t.Fatalf("ReadRecord = %+v, %t, %v", got, found, err)
	}
}

// writeRecordFile writes the bytes of a record of the fixture name by hand.
func writeRecordFile(t *testing.T, state, content string) {
	t.Helper()
	path := profile.RecordPath(state, "fixture")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestReadRecordRefusesADamagedRecord(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		content string
		want    string
	}{
		"not json":         {"{", "unexpected end of JSON input"},
		"another schema":   {`{"golaw": 2, "name": "fixture", "current": "1.2.3", "versions": [{"version": "1.2.3"}]}`, "golaw is 2, not 1"},
		"another name":     {`{"golaw": 1, "name": "other", "current": "1.2.3", "versions": [{"version": "1.2.3"}]}`, `name is "other", not "fixture"`},
		"no versions":      {`{"golaw": 1, "name": "fixture", "current": "1.2.3", "versions": []}`, `current "1.2.3" is not the last installed version`},
		"current not last": {`{"golaw": 1, "name": "fixture", "current": "1.2.3", "versions": [{"version": "1.2.3"}, {"version": "1.3.0"}]}`, `current "1.2.3" is not the last`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			state := t.TempDir()
			writeRecordFile(t, state, tc.content)
			_, found, err := profile.ReadRecord(state, "fixture")
			if !errors.Is(err, profile.ErrBundle) || !errors.Is(err, model.ErrPolicy) || found ||
				!strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "run golaw profile remove fixture, then install it again") {
				t.Fatalf("ReadRecord = %t, %v, want %q", found, err, tc.want)
			}
		})
	}
}

// The near miss of the order rule: the current version last is sound.
func TestReadRecordAcceptsTheCurrentVersionLast(t *testing.T) {
	t.Parallel()
	state := t.TempDir()
	writeRecordFile(t, state, `{"golaw": 1, "name": "fixture", "current": "1.3.0", "versions": [{"version": "1.2.3"}, {"version": "1.3.0"}]}`)
	if r, found, err := profile.ReadRecord(state, "fixture"); err != nil || !found || r.Current != "1.3.0" {
		t.Fatalf("a sound record = %+v, %t, %v", r, found, err)
	}
}

func TestReadRecordNamesAnUnreadableRecord(t *testing.T) {
	t.Parallel()
	state := t.TempDir()
	if err := os.MkdirAll(profile.RecordPath(state, "fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := profile.ReadRecord(state, "fixture"); !errors.Is(err, profile.ErrBundle) || !strings.Contains(err.Error(), "fixture.json") {
		t.Fatalf("a directory in place of the record = %v", err)
	}
}

func TestWriteRecordRefusesWhatItCannotStore(t *testing.T) {
	t.Parallel()
	blocked := filepath.Join(t.TempDir(), "state")
	if err := os.WriteFile(blocked, []byte("a file where the state directory goes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sound := profile.Record{Name: "fixture", Current: "1.2.3", Versions: []profile.Installed{installedAt("1.2.3", "sha256:aa", now())}}
	cases := map[string]struct {
		state  string
		record profile.Record
		usage  bool
	}{
		"no versions":      {t.TempDir(), profile.Record{Name: "fixture", Current: "1.2.3"}, true},
		"a bad name":       {t.TempDir(), profile.Record{Name: "Fixture", Current: "1.2.3", Versions: sound.Versions}, true},
		"current not last": {t.TempDir(), profile.Record{Name: "fixture", Current: "1.0.0", Versions: sound.Versions}, true},
		"state is a file":  {blocked, sound, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := profile.WriteRecord(tc.state, tc.record)
			if err == nil || errors.Is(err, model.ErrUsage) != tc.usage {
				t.Fatalf("WriteRecord = %v, usage %t", err, tc.usage)
			}
		})
	}
}
