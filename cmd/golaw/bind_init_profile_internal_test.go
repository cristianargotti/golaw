package main

import (
	"io/fs"
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

// birthOwners is the CODEOWNERS of a birth whose catch-all team owns
// .github/ too, so only the profile can leave it out.
const birthOwners = "* @example-org/service-team\n"

// fixtureInstallCommand is what every refusal of a missing or mismatched fixture
// bundle names (A19).
const fixtureInstallCommand = "run golaw profile install " + profiletest.Source + "@" + profiletest.Version

// birthRepository commits the template birth of follow-up 7 that
// tools/acceptance holds, with the CODEOWNERS given in place of its own
// when owners is not empty.
func birthRepository(t *testing.T, owners string) *gittest.Repo {
	t.Helper()
	birth := os.DirFS(filepath.Join(profiletest.Dir(t), "..", "..", "..", "..", "tools", "acceptance", "testdata", "m3", "template", "birth"))
	files := map[string]string{}
	err := fs.WalkDir(birth, ".", func(rel string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || !entry.Type().IsRegular() {
			return walkErr
		}
		data, readErr := fs.ReadFile(birth, rel)
		if readErr != nil {
			t.Fatal(readErr)
		}
		files[strings.TrimSuffix(rel, ".txt")] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if owners != "" {
		files[".github/CODEOWNERS"] = owners
	}
	repo := gittest.New(t)
	repo.Commit("chore: the template birth", files)
	return repo
}

// profiledBindApp is initBindApp with the fixture bundle installed under
// the config directory of its binding.
func profiledBindApp(t *testing.T) testApp {
	t.Helper()
	ta := initBindApp(t)
	dirs, err := ta.bind().dirs()
	if err != nil {
		t.Fatal(err)
	}
	profiletest.Install(t, dirs.Config)
	return ta
}

// githubFiles lists the files under .github/ of a root.
func githubFiles(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := fs.WalkDir(os.DirFS(root), ".github", func(rel string, entry fs.DirEntry, walkErr error) error {
		if walkErr == nil && entry.Type().IsRegular() {
			out = append(out, rel)
		}
		return walkErr
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// layoutBundle installs the fixture bundle with the layout defaults given
// in its rules.yml and the stamp carrying the digest of that content.
func layoutBundle(t *testing.T, configDir, layout string) {
	t.Helper()
	stamp := profiletest.Install(t, configDir)
	dir := profile.Dir(configDir, profiletest.Name)
	rules := filepath.Join(dir, profile.RulesFile)
	data, err := os.ReadFile(rules)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(rules, []byte(strings.Replace(string(data), "layout: { main: root, decisions: docs/adr }", layout, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	digest, err := profile.Digest(dir)
	if err != nil {
		t.Fatal(err)
	}
	stampPath := policy.StampPath(configDir, profiletest.Name)
	text, err := os.ReadFile(stampPath)
	if err == nil {
		err = os.WriteFile(stampPath, []byte(strings.Replace(string(text), stamp.Digest, digest, 1)), 0o600)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestInitUnderTheProfileLeavesGithubToThePlatform(t *testing.T) {
	t.Parallel()
	for name, owners := range map[string]string{"a foreign .github": "", "a catch-all owner": birthOwners} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			repo := birthRepository(t, owners)
			ta := profiledBindApp(t)
			if code := ta.run(t.Context(), []string{"init", "--apply", "--profile", profiletest.Name, repo.Root}); code != exitClean {
				t.Fatalf("exit = %d: %s", code, ta.stderr.String())
			}
			if got := githubFiles(t, repo.Root); !slices.Equal(got, []string{".github/CODEOWNERS"}) {
				t.Errorf("init wrote under .github/: %v", got)
			}
			if out := ta.stdout.String(); !strings.Contains(out, "Deixados para a plataforma: .github/dependabot.yml") || strings.Contains(out, "+++ .github/") {
				t.Errorf("stdout:\n%s", out)
			}
			f := seededPolicy(t, repo.Root)
			if f.Profile == nil || *f.Profile != profiletest.Ref(t) ||
				slices.ContainsFunc(f.Managed, func(m policy.ManagedFile) bool { return strings.HasPrefix(m.Path, ".github/") }) {
				t.Errorf("profile = %+v, managed = %+v", f.Profile, f.Managed)
			}
		})
	}
}

func TestInitWithoutTheProfileWritesGithubForACatchAllOwner(t *testing.T) {
	t.Parallel()
	repo := birthRepository(t, birthOwners)
	ta := initBindApp(t)
	if code := ta.run(t.Context(), []string{"init", "--apply", repo.Root}); code != exitClean {
		t.Fatalf("exit = %d: %s", code, ta.stderr.String())
	}
	if got := githubFiles(t, repo.Root); !slices.Contains(got, ".github/dependabot.yml") || strings.Contains(ta.stdout.String(), "Deixados para a plataforma") {
		t.Errorf("a core adoption left .github/ out: %v\n%s", got, ta.stdout.String())
	}
}

func TestInitUpdateKeepsGithubOutUnderTheProfile(t *testing.T) {
	t.Parallel()
	repo := birthRepository(t, birthOwners)
	ta := profiledBindApp(t)
	if code := ta.run(t.Context(), []string{"init", "--apply", "--profile", profiletest.Name, repo.Root}); code != exitClean {
		t.Fatalf("init exit = %d: %s", code, ta.stderr.String())
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "build: adopt golaw under the profile")
	ta.stdout.Reset()
	if code := ta.run(t.Context(), []string{"init", "--update", "--apply", repo.Root}); code != exitClean {
		t.Fatalf("update exit = %d: %s", code, ta.stderr.String())
	}
	if got := githubFiles(t, repo.Root); !slices.Equal(got, []string{".github/CODEOWNERS"}) || !strings.Contains(ta.stdout.String(), "Deixados para a plataforma: .github/dependabot.yml") {
		t.Errorf("the update wrote under .github/: %v\n%s", got, ta.stdout.String())
	}
}

// requireBundleRefusal runs init over a repository whose golaw.yml pins the
// fixture bundle and requires one stderr line naming the install command,
// nothing on stdout and no template written.
func requireBundleRefusal(t *testing.T, ta testApp, args []string) {
	t.Helper()
	root := profiledRepository(t)
	if code := ta.run(t.Context(), append(slices.Clone(args), root)); code != exitFailure {
		t.Fatalf("exit = %d, stdout %q", code, ta.stdout.String())
	}
	lines := strings.Split(strings.TrimSuffix(ta.stderr.String(), "\n"), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], fixtureInstallCommand) || ta.stdout.Len() != 0 {
		t.Errorf("stderr %q, stdout %q", ta.stderr.String(), ta.stdout.String())
	}
	if _, err := os.Stat(filepath.Join(root, "lefthook.yml")); err == nil {
		t.Error("init wrote a template over a bundle it could not load")
	}
}

func TestInitRefusesABundleItCannotLoadWithTheInstallCommand(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		args     []string
		tampered bool
	}{
		{"red apply missing", []string{"init", "--apply"}, false},
		{"red apply tampered", []string{"init", "--apply"}, true},
		{"red preview missing", []string{"init"}, false},
		{"red update tampered", []string{"init", "--update", "--apply"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ta := initBindApp(t)
			if tc.tampered {
				dirs, err := ta.bind().dirs()
				if err != nil {
					t.Fatal(err)
				}
				profiletest.Tampered(t, dirs.Config)
			}
			requireBundleRefusal(t, ta, tc.args)
		})
	}
	ta := profiledBindApp(t)
	if code := ta.run(t.Context(), []string{"init", "--apply", profiledRepository(t)}); code != exitClean {
		t.Errorf("near the installed bundle: exit %d: %s", code, ta.stderr.String())
	}
}

func TestInitRendersTheLayoutDefaultsOfTheProfile(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		entries map[string]string
		want    string
	}{
		{"red no layout in the tree", nil, "layout: { main: cmd, decisions: docs/decisions }\n"},
		{"near the tree shows its layout", map[string]string{"main.go": "package main\n\nfunc main() {}\n", "docs/adr/0000-start.md": "# start\n"}, "layout: { main: root, decisions: docs/adr }\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ta, _ := initApp(t)
			dirs, err := ta.bind().dirs()
			if err != nil {
				t.Fatal(err)
			}
			layoutBundle(t, dirs.Config, "layout: { main: cmd, decisions: docs/decisions }")
			root := checkout(t)
			writeFiles(t, root, tc.entries)
			if code := ta.run(t.Context(), []string{"init", "--profile", profiletest.Name, root}); code != exitClean {
				t.Fatalf("exit = %d: %s", code, ta.stderr.String())
			}
			if !strings.Contains(ta.stdout.String(), "+"+tc.want) {
				t.Errorf("the rendered policy lacks %q:\n%s", tc.want, ta.stdout.String())
			}
		})
	}
}

func TestInitSeedsWhatTheProfileFindsUnderTheProfile(t *testing.T) {
	t.Parallel()
	term := profiletest.Data(t).Terms[0]
	want := policy.ExceptionRule{Check: "text.tool-terms", Path: "notes.md", Record: seedRecord, Until: seedUntil}
	for name, args := range map[string][]string{
		"red under the profile": {"init", "--apply", "--seed-debt", "--profile", profiletest.Name},
		"near core only":        {"init", "--apply", "--seed-debt"},
	} {
		repo := seedRepository(t, map[string]string{"notes.md": "built by " + term + "\n"})
		ta := profiledBindApp(t)
		ta.registry, ta.snapshot = seedApp(t).registry, ta.bind().snapshot
		if code := ta.run(t.Context(), append(args, repo.Root)); code != exitClean {
			t.Fatalf("%s exit = %d: %s", name, code, ta.stderr.String())
		}
		if got := slices.Contains(seededPolicy(t, repo.Root).Exceptions, want); got != strings.HasPrefix(name, "red") {
			t.Errorf("%s: the profile term seeded = %v", name, got)
		}
	}
}
