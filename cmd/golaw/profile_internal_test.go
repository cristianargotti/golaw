package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/profiletest"
)

func TestProfileCommandNeedsASubcommand(t *testing.T) {
	t.Parallel()
	ta := bindApp(t)
	if got := ta.run(t.Context(), []string{"profile"}); got != exitFailure {
		t.Fatalf("exit = %d", got)
	}
	if got := ta.stderr.String(); !strings.Contains(got, "profile needs a subcommand (install, refresh, rollback, remove, digest)") {
		t.Fatalf("stderr = %q", got)
	}
}

func TestProfileDigestPrintsTheDigestOfABundle(t *testing.T) {
	t.Parallel()
	ta := bindApp(t)
	if got := ta.run(t.Context(), []string{"profile", "digest", profiletest.Dir(t)}); got != exitClean {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	if got, want := ta.stdout.String(), profiletest.Ref(t).Digest+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestProfileDigestRefusesWhatIsNotABundle(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		files map[string]string
		want  string
	}{
		"no rules.yml":   {map[string]string{"ONBOARDING.md": "text\n"}, "rules.yml"},
		"an unknown key": {map[string]string{"rules.yml": "golaw_profile: 1\nextra: 1\n"}, "extra"},
		"a bad term":     {map[string]string{"rules.yml": "golaw_profile: 1\nterms: [Legacy_CI]\n"}, "Legacy_CI"},
		"the branch key": {map[string]string{"rules.yml": "golaw_profile: 1\nbranch: x\n"}, "branch"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ta := bindApp(t)
			dir := t.TempDir()
			writeFiles(t, dir, tc.files)
			if got := ta.run(t.Context(), []string{"profile", "digest", dir}); got != exitFailure {
				t.Fatalf("exit = %d", got)
			}
			if got := ta.stderr.String(); !strings.HasPrefix(got, "golaw: profile digest: ") || !strings.Contains(got, tc.want) || ta.stdout.Len() != 0 {
				t.Fatalf("stderr = %q, stdout = %q", got, ta.stdout.String())
			}
		})
	}
}

// The lifecycle exits 2 with one line naming its subcommand and its cause,
// and prints nothing on stdout.
func TestProfileLifecycleFailuresExitTwoWithOneLine(t *testing.T) {
	t.Parallel()
	absent := filepath.Join(t.TempDir(), "absent.git")
	// git clone reads a missing path that holds a colon, a Windows drive
	// letter included, as a URL, and then its transport names the path.
	noSource := "does not exist"
	if strings.Contains(absent, ":") {
		noSource = "does not appear to be a git repository"
	}
	cases := map[string]struct {
		args []string
		want string
	}{
		"install without a source": {[]string{"profile", "install"}, "golaw: accepts 1 arg(s), received 0"},
		"install of a bad source":  {[]string{"profile", "install", "https://example.test/profiles/fixture"}, "is not <git url>@<tag>"},
		"install of a bad tag":     {[]string{"profile", "install", absent + "@latest"}, `tag "latest" is not vX.Y.Z or X.Y.Z`},
		"install of no source":     {[]string{"profile", "install", absent + "@v1.2.3"}, noSource},
		"refresh":                  {[]string{"profile", "refresh", profiletest.Name}, "golaw: profile refresh: profile fixture has no install record"},
		"rollback":                 {[]string{"profile", "rollback", profiletest.Name}, "golaw: profile rollback: profile fixture has no install record"},
		"remove":                   {[]string{"profile", "remove", profiletest.Name}, "golaw: profile remove: profile fixture is not installed"},
		"remove of a path":         {[]string{"profile", "remove", "../fixture"}, `"../fixture" is not a profile name`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ta, _ := lifecycleApp(t)
			if got := ta.run(t.Context(), tc.args); got != exitFailure {
				t.Fatalf("exit = %d", got)
			}
			got := ta.stderr.String()
			// The line names the cause, never the sentinels of the policy
			// chain, because no policy is invalid when a step fails.
			if strings.Contains(got, "invalid policy") || strings.Contains(got, "invalid profile bundle") {
				t.Fatalf("stderr names a sentinel: %q", got)
			}
			if !strings.HasPrefix(got, "golaw: ") || !strings.Contains(got, tc.want) || strings.Count(got, "\n") != 1 || ta.stdout.Len() != 0 {
				t.Fatalf("stderr = %q, stdout = %q", got, ta.stdout.String())
			}
		})
	}
}

// A machine whose directories cannot be resolved gets the same one line
// from every lifecycle step, naming the step and the variable to set.
func TestProfileLifecycleNamesUnresolvedDirectories(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"profile", "install", "https://example.test/profiles/fixture@v1.2.3"},
		{"profile", "refresh", profiletest.Name},
		{"profile", "rollback", profiletest.Name},
		{"profile", "remove", profiletest.Name},
	} {
		t.Run(args[1], func(t *testing.T) {
			t.Parallel()
			ta := bindApp(t)
			unset := ta.bind()
			unset.env = func(string) string { return "" }
			ta.bind = func() binding { return unset }
			if got := ta.run(t.Context(), args); got != exitFailure {
				t.Fatalf("exit = %d", got)
			}
			want := "golaw: profile " + args[1] + ": resolve the state directories: "
			if got := ta.stderr.String(); !strings.HasPrefix(got, want) || !strings.Contains(got, "HOME") || strings.Count(got, "\n") != 1 || ta.stdout.Len() != 0 {
				t.Fatalf("stderr = %q, stdout = %q", got, ta.stdout.String())
			}
		})
	}
}
