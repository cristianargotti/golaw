package main

import (
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

// profileClock is two days after the date stamp of the fixture bundle.
func profileClock() time.Time { return time.Date(2026, time.October, 22, 9, 0, 0, 0, time.UTC) }

func TestProfileLineReportsThePinBesideTheBundle(t *testing.T) {
	t.Parallel()
	install := "run golaw profile install " + profiletest.Source + "@" + profiletest.Version
	cases := map[string]struct {
		root    func(t *testing.T, config string) string
		version string
		state   string
		code    string
		detail  string
	}{
		"core only": {func(t *testing.T, _ string) string {
			t.Helper()
			return repository(t, true)
		}, testVersion, "ok", "profile-none", "no profile"},
		"installed": {func(t *testing.T, config string) string {
			t.Helper()
			profiletest.Install(t, config)
			return profiledRepository(t)
		}, testVersion, "ok", "profile-installed", "version 1.2.3, digest " + profiletest.Ref(t).Digest + ", 2 days since 2026-10-20"},
		"not installed": {func(t *testing.T, _ string) string {
			t.Helper()
			return profiledRepository(t)
		}, testVersion, "red", "profile-not-installed", install},
		"tampered": {func(t *testing.T, config string) string {
			t.Helper()
			profiletest.Tampered(t, config)
			return profiledRepository(t)
		}, testVersion, "red", "profile-not-installed", "differs from its stamp"},
		"a load error elsewhere": {func(t *testing.T, config string) string {
			t.Helper()
			profiletest.Install(t, config)
			return profiledRepository(t)
		}, "9.9.8", "ok", "profile-installed", "version 1.2.3"},
	}
	for name, tc := range cases {
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
			if line.ID != "doctor.profile" || line.State != tc.state || line.Code != tc.code || !strings.Contains(line.Detail, tc.detail) {
				t.Fatalf("line = %+v", line)
			}
		})
	}
}

func TestDoctorJoinsTheProfileLineToTheFirstReport(t *testing.T) {
	t.Parallel()
	ta, _ := doctorApp(t)
	root := profiledRepository(t)
	if got := ta.run(t.Context(), []string{"doctor", root}); got != exitClean {
		t.Fatalf("a red profile line keeps exit 0: exit = %d, stderr = %q", got, ta.stderr.String())
	}
	if out := ta.stdout.String(); !strings.Contains(out, "doctor.profile") || !strings.Contains(out, "profile-not-installed") {
		t.Fatalf("stdout = %q", out)
	}
	var none doctor.Line
	if none = ta.bind().profileDoctor(t.Context(), t.TempDir(), profileClock()); none.Code != "profile-none" {
		t.Fatalf("outside git = %+v", none)
	}
}
