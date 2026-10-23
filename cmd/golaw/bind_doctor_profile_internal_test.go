package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

// profileClock is two days after the date stamp of the fixture bundle.
func profileClock() time.Time { return time.Date(2026, time.October, 22, 9, 0, 0, 0, time.UTC) }

// placeBundle writes a bundle by hand into dir, the way a config directory
// may hold one: the fixture's files with some replaced, and a stamp of the
// version and source given carrying the digest the files hash to, which it
// answers.
func placeBundle(t *testing.T, dir, version, source string, files map[string]string) string {
	t.Helper()
	contents := map[string]string{}
	for _, name := range []string{profile.RulesFile, "ONBOARDING.md"} {
		data, err := os.ReadFile(filepath.Join(profiletest.Dir(t), name))
		if err != nil {
			t.Fatal(err)
		}
		contents[name] = string(data)
	}
	maps.Copy(contents, files)
	writeFiles(t, dir, contents)
	digest, err := profile.Digest(dir)
	if err != nil {
		t.Fatal(err)
	}
	stamp := fmt.Sprintf("name: fixture\nversion: %s\ndate: 2026-10-20\nsource: %s\ndigest: %s\n", version, source, digest)
	writeFiles(t, dir, map[string]string{policy.StampFile: stamp})
	return digest
}

// repinnedRepository is a profiled repository whose pin moved to ref and
// was committed, the way init --profile --apply and a commit move it.
func repinnedRepository(t *testing.T, ref policy.ProfileRef) string {
	t.Helper()
	root := profiledRepository(t)
	current, err := os.ReadFile(filepath.Join(root, policy.FileName))
	if err != nil {
		t.Fatal(err)
	}
	next, err := bootstrap.SetProfile(current, ref)
	if err != nil {
		t.Fatal(err)
	}
	commitFiles(t, root, map[string]string{policy.FileName: string(next)})
	return root
}

// profileCase is one state of the config directory and the line it gives.
type profileCase struct {
	root    func(t *testing.T, config string) string
	version string
	state   string
	code    string
	detail  string
}

// placed is a red case whose current bundle is written by hand and whose
// repository pins the fixture.
func placed(version, source string, files map[string]string, code, detail string) profileCase {
	return profileCase{func(t *testing.T, config string) string {
		t.Helper()
		placeBundle(t, profile.Dir(config, profiletest.Name), version, source, files)
		return profiledRepository(t)
	}, testVersion, "red", code, detail}
}

// profileCases are the states the profile line tells apart.
func profileCases(t *testing.T) map[string]profileCase {
	t.Helper()
	install := "run golaw profile install " + profiletest.Source + "@" + profiletest.Version
	second := map[string]string{"ONBOARDING.md": "the second version\n"}
	return map[string]profileCase{
		"core only": {func(t *testing.T, _ string) string {
			t.Helper()
			return repository(t, true)
		}, testVersion, "ok", "profile-none", "no profile"},
		"installed": {func(t *testing.T, config string) string {
			t.Helper()
			profiletest.Install(t, config)
			return profiledRepository(t)
		}, testVersion, "ok", "profile-installed", "version 1.2.3, digest " + profiletest.Ref(t).Digest + ", 2 days since 2026-10-20"},
		"installed in the store only": {func(t *testing.T, config string) string {
			t.Helper()
			placeBundle(t, profile.Dir(config, profiletest.Name), "1.3.0", profiletest.Source, second)
			placeBundle(t, filepath.Dir(policy.StoredStampPath(config, profiletest.Name, profiletest.Version)), profiletest.Version, profiletest.Source, nil)
			return profiledRepository(t)
		}, testVersion, "ok", "profile-installed", "version 1.2.3"},
		"not installed": {func(t *testing.T, _ string) string {
			t.Helper()
			return profiledRepository(t)
		}, testVersion, "red", "profile-not-installed", install},
		"another source under the name":        placed("1.2.3", "https://example.test/profiles/other", nil, "profile-not-installed", "source is"),
		"ahead of the pin":                     placed("1.3.0", profiletest.Source, second, "profile-version-mismatch", install),
		"behind the pin":                       placed("1.1.0", profiletest.Source, second, "profile-stale", install),
		"another digest at the pinned version": placed("1.2.3", profiletest.Source, second, "profile-digest-mismatch", install),
		"tampered": {func(t *testing.T, config string) string {
			t.Helper()
			profiletest.Tampered(t, config)
			return profiledRepository(t)
		}, testVersion, "red", "profile-digest-mismatch", "differs from its stamp"},
		"data the loader refuses": {func(t *testing.T, config string) string {
			t.Helper()
			digest := placeBundle(t, profile.Dir(config, profiletest.Name), "1.2.3", profiletest.Source, map[string]string{profile.RulesFile: "golaw_profile: 1\nextra: 1\n"})
			ref := profiletest.Ref(t)
			ref.Digest = digest
			return repinnedRepository(t, ref)
		}, testVersion, "red", "profile-data-invalid", "rules.yml"},
		"a load error elsewhere": {func(t *testing.T, config string) string {
			t.Helper()
			profiletest.Install(t, config)
			return profiledRepository(t)
		}, "9.9.8", "ok", "profile-installed", "version 1.2.3"},
	}
}

func TestProfileLineReportsThePinBesideTheBundle(t *testing.T) {
	t.Parallel()
	for name, tc := range profileCases(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b, _ := testBinding(t)
			dirs, err := b.dirs()
			if err != nil {
				t.Fatal(err)
			}
			root := tc.root(t, dirs.Config)
			b.info.Version = tc.version
			line := b.profileDoctor(t.Context(), root, profileClock())
			if line.ID != "doctor.profile" || line.State != tc.state || line.Code != tc.code || !strings.Contains(line.Detail, tc.detail) ||
				strings.HasPrefix(line.Detail, "invalid policy") {
				t.Fatalf("line = %+v", line)
			}
		})
	}
}

func TestProfileLineNamesAConfigDirectoryItCannotResolve(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	root := profiledRepository(t)
	b.env = func(string) string { return "" }
	if line := b.profileDoctor(t.Context(), root, profileClock()); line.State != "red" || line.Code != "profile-not-installed" || line.Detail == "" {
		t.Fatalf("line = %+v", line)
	}
}

func TestDoctorJoinsTheProfileLineToTheFirstReport(t *testing.T) {
	t.Parallel()
	ta, _ := doctorApp(t)
	dirs, err := ta.bind().dirs()
	if err != nil {
		t.Fatal(err)
	}
	placeBundle(t, profile.Dir(dirs.Config, profiletest.Name), "1.1.0", profiletest.Source, map[string]string{"ONBOARDING.md": "an older version\n"})
	root := profiledRepository(t)
	if got := ta.run(t.Context(), []string{"doctor", "--json", root}); got != exitClean {
		t.Fatalf("a red profile line keeps exit 0: exit = %d, stderr = %q", got, ta.stderr.String())
	}
	decoder := json.NewDecoder(strings.NewReader(ta.stdout.String()))
	var first doctor.Report
	if err := decoder.Decode(&first); err != nil {
		t.Fatal(err)
	}
	for {
		var next doctor.Report
		if err := decoder.Decode(&next); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
	}
	found := false
	for _, line := range first.Lines {
		found = found || (line.ID == "doctor.profile" && line.Code == "profile-stale")
	}
	if !found {
		t.Fatalf("the first report carries the stale profile line: %+v", first.Lines)
	}
	if none := ta.bind().profileDoctor(t.Context(), t.TempDir(), profileClock()); none.Code != "profile-none" {
		t.Fatalf("outside git = %+v", none)
	}
}
