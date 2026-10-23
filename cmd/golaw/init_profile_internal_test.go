package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

// profiledInitApp is initApp with the fixture bundle installed under the
// config directory of its binding, the state init --profile reads.
func profiledInitApp(t *testing.T) (testApp, *enrollment, policy.ProfileRef) {
	t.Helper()
	ta, record := initApp(t)
	dirs, err := ta.bind().dirs()
	if err != nil {
		t.Fatal(err)
	}
	profiletest.Install(t, dirs.Config)
	return ta, record, profiletest.Ref(t)
}

// pinLine is the flow line init writes for a reference.
func pinLine(ref policy.ProfileRef) string {
	return "profile: { name: " + ref.Name + ", source: " + ref.Source + ", version: " + ref.Version + ", digest: " + ref.Digest + " }"
}

func TestInitProfilePinsTheInstalledBundleOfANewPolicy(t *testing.T) {
	t.Parallel()
	ta, record, ref := profiledInitApp(t)
	root := checkout(t)
	if got := ta.run(t.Context(), []string{"init", "--profile", "fixture", root}); got != exitClean {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	if out := ta.stdout.String(); !strings.Contains(out, "+"+pinLine(ref)+"\n") || record.calls != 0 {
		t.Fatalf("stdout = %q, enrollment = %+v", out, record)
	}
	ta.stdout.Reset()
	if got := ta.run(t.Context(), []string{"init", "--apply", "--profile", "fixture", root}); got != exitClean {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	data, err := os.ReadFile(filepath.Join(root, bootstrap.PolicyFile))
	if err != nil {
		t.Fatal(err)
	}
	f, err := policy.Decode(data)
	if err != nil || f.Profile == nil || *f.Profile != ref || record.calls != 1 {
		t.Fatalf("written profile = %+v, %v, enrollment = %+v", f.Profile, err, record)
	}
}

func TestInitProfileEnrollsAnExistingPolicy(t *testing.T) {
	t.Parallel()
	ta, _, ref := profiledInitApp(t)
	root := checkout(t)
	if err := os.WriteFile(filepath.Join(root, bootstrap.PolicyFile), renderedPolicy(t), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := ta.run(t.Context(), []string{"init", "--apply", "--profile", "fixture", root}); got != exitClean {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	data, err := os.ReadFile(filepath.Join(root, bootstrap.PolicyFile))
	if err != nil {
		t.Fatal(err)
	}
	// init records the managed templates of an existing policy as well, so
	// the pin is read in its place and the rest of the file by its decode.
	want := "kit: { version: " + testVersion + " }\n" + pinLine(ref) + "\nlanguage: pt-BR\n"
	f, err := policy.Decode(data)
	if err != nil || f.Profile == nil || *f.Profile != ref || !strings.Contains(string(data), want) {
		t.Fatalf("policy:\n%s\nwant %q in it, %v", data, want, err)
	}
}

func TestInitProfileRefusesWhatItCannotPin(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		args []string
		want string
	}{
		"no bundle":    {[]string{"--apply", "--profile", "fixture"}, "init: usage: profile fixture is not installed: run golaw profile install <git url>@<tag>"},
		"with update":  {[]string{"--update", "--profile", "fixture"}, "init: usage: --profile enrolls a pin, while init --update keeps the policy and moves the kit pin"},
		"not a name":   {[]string{"--profile", "../fixture"}, `init: usage: profile name "../fixture" is not a lower case name`},
		"another name": {[]string{"--profile", "other"}, "init: usage: the bundle under"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ta, record := initApp(t)
			dirs, err := ta.bind().dirs()
			if err != nil {
				t.Fatal(err)
			}
			copyFixtureAs(t, profile.Dir(dirs.Config, "other"))
			root := checkout(t)
			if got := ta.run(t.Context(), append([]string{"init"}, append(tc.args, root)...)); got != exitFailure {
				t.Fatalf("exit = %d, stdout = %q", got, ta.stdout.String())
			}
			if !strings.Contains(ta.stderr.String(), tc.want) || record.calls != 0 {
				t.Fatalf("stderr = %q, enrollment = %+v", ta.stderr.String(), record)
			}
			if _, err := os.Stat(filepath.Join(root, bootstrap.PolicyFile)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("policy written: %v", err)
			}
		})
	}
}

// A teammate on a fresh machine who runs init --profile in a clone whose
// golaw.yml already pins the name reads the exact command of the pin; the
// "no bundle" row above, a first pin, keeps the form.
func TestInitProfileNamesTheCommandOfThePinAtHead(t *testing.T) {
	t.Parallel()
	ta, record := initApp(t)
	root := profiledRepository(t)
	if got := ta.run(t.Context(), []string{"init", "--apply", "--profile", "fixture", root}); got != exitFailure {
		t.Fatalf("exit = %d, stdout = %q", got, ta.stdout.String())
	}
	want := "profile fixture is not installed: run golaw profile install " + profiletest.Source + "@" + profiletest.Version
	if got := ta.stderr.String(); !strings.Contains(got, want) || record.calls != 0 {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

// copyFixtureAs copies the fixture bundle into a directory of another
// name, the state of a stamp whose name differs from its directory.
func copyFixtureAs(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(profiletest.Dir(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(profiletest.Dir(t), entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, entry.Name()), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestProfilePinReadsTheStampOfTheCurrentBundle(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	dirs, err := b.dirs()
	if err != nil {
		t.Fatal(err)
	}
	if _, absent := b.profilePin(t.Context(), t.TempDir(), "fixture"); !errors.Is(absent, model.ErrUsage) {
		t.Fatalf("an absent bundle = %v", absent)
	}
	profiletest.Install(t, dirs.Config)
	got, err := b.profilePin(t.Context(), t.TempDir(), "fixture")
	if err != nil || got != profiletest.Ref(t) {
		t.Fatalf("pin = %+v, %v", got, err)
	}
	if err = os.WriteFile(policy.StampPath(dirs.Config, "fixture"), []byte("name: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, unreadable := b.profilePin(t.Context(), t.TempDir(), "fixture"); unreadable == nil || errors.Is(unreadable, model.ErrUsage) || !strings.Contains(unreadable.Error(), "read the stamp of profile fixture") {
		t.Fatalf("an unreadable stamp = %v", unreadable)
	}
	homeless := b
	homeless.env = func(string) string { return "" }
	if _, noHome := homeless.profilePin(t.Context(), t.TempDir(), "fixture"); noHome == nil {
		t.Fatal("a pin was read without a config directory")
	}
}
