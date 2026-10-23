package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

// lifecycleApp is the test app over a temporary home whose git runs with
// the isolated environment of the tests, so a clone of a local bare
// repository works the same on every system and reads no person's
// configuration.
func lifecycleApp(t *testing.T) (testApp, policy.Dirs) {
	t.Helper()
	ta := newTestApp(t)
	b, _ := testBinding(t)
	b.environ = gittest.Isolated
	ta.bind = func() binding { return b }
	dirs, err := b.dirs()
	if err != nil {
		t.Fatal(err)
	}
	return ta, dirs
}

// twoTagSource is a local bare repository serving the fixture at v1.2.3
// and a 1.3.0 whose onboarding page differs, the two versions a rollback
// moves between; no test reaches the network.
func twoTagSource(t *testing.T) string {
	t.Helper()
	work := gittest.New(t)
	files := map[string]string{}
	for _, name := range []string{policy.StampFile, profile.RulesFile, "ONBOARDING.md"} {
		data, err := os.ReadFile(filepath.Join(profiletest.Dir(t), name))
		if err != nil {
			t.Fatal(err)
		}
		files[name] = string(data)
	}
	work.Commit("feat: the fixture bundle", files)
	work.Git("tag", profiletest.Tag)
	work.Remove(policy.StampFile)
	work.Write("ONBOARDING.md", "the second version\n")
	digest, err := profile.Digest(work.Root)
	if err != nil {
		t.Fatal(err)
	}
	work.Commit("feat: the second version", map[string]string{
		"ONBOARDING.md":  "the second version\n",
		policy.StampFile: fmt.Sprintf("name: fixture\nversion: 1.3.0\ndate: 2026-10-21\nsource: %s\ndigest: %s\n", profiletest.Source, digest),
	})
	work.Git("tag", "v1.3.0")
	bare := filepath.Join(t.TempDir(), "source.git")
	work.Git("clone", "-q", "--bare", work.Root, bare)
	return bare
}

// lifecycleStep is one command of the lifecycle and the stdout it prints.
type lifecycleStep struct {
	args []string
	want []string
}

func TestProfileLifecycleNeverTouchesThePin(t *testing.T) {
	t.Parallel()
	ta, _ := lifecycleApp(t)
	root := profiledRepository(t)
	pinned, err := os.ReadFile(filepath.Join(root, policy.FileName))
	if err != nil {
		t.Fatal(err)
	}
	url := twoTagSource(t)
	digest := profiletest.Ref(t).Digest
	source := "  source: " + profiletest.Source + " (the source golaw.yml pins)"
	pin := func(version string) string {
		return "golaw.yml não mudou: para fixar fixture " + version + " em um repositório, execute golaw init --profile fixture --apply e faça commit de golaw.yml"
	}
	steps := []lifecycleStep{
		{[]string{"profile", "install", url + "@v1.2.3"}, []string{"Perfil fixture 1.2.3 instalado de " + url + "@v1.2.3, digest " + digest, source, pin("1.2.3")}},
		{[]string{"profile", "install", url + "@v1.2.3"}, []string{"Perfil fixture 1.2.3 já estava instalado"}},
		{[]string{"profile", "install", url + "@v1.3.0"}, nil},
		{[]string{"profile", "refresh", "fixture"}, nil},
		{[]string{"profile", "rollback", "fixture"}, []string{"Perfil fixture voltou para 1.2.3", pin("1.2.3")}},
		{[]string{"profile", "remove", "fixture"}, []string{"Perfil fixture removido"}},
	}
	for _, step := range steps {
		ta.stdout.Reset()
		if got := ta.run(t.Context(), step.args); got != exitClean || ta.stderr.Len() != 0 {
			t.Fatalf("%v: exit %d, stderr %q", step.args, got, ta.stderr.String())
		}
		if step.want != nil && ta.stdout.String() != strings.Join(step.want, "\n")+"\n" {
			t.Fatalf("%v: stdout %q, want %q", step.args, ta.stdout.String(), step.want)
		}
		if after, err := os.ReadFile(filepath.Join(root, policy.FileName)); err != nil || !bytes.Equal(after, pinned) {
			t.Fatalf("%v changed golaw.yml: %v", step.args, err)
		}
	}
}

func TestProfileInstallMakesTheRepositoryLoad(t *testing.T) {
	t.Parallel()
	ta, dirs := lifecycleApp(t)
	root := profiledRepository(t)
	if _, err := ta.bind().load(t.Context(), root, now()); err == nil {
		t.Fatal("a pinned repository loads before the install")
	}
	url, tag := profiletest.BareSource(t)
	if got := ta.run(t.Context(), []string{"profile", "install", url + "@" + tag}); got != exitClean {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	if p, err := ta.bind().load(t.Context(), root, now()); err != nil || p.ProfileData == nil {
		t.Fatalf("the pinned repository loads its bundle: %v", err)
	}
	if _, err := os.Stat(profile.RecordPath(dirs.State, profiletest.Name)); err != nil {
		t.Fatalf("the install wrote its record: %v", err)
	}
}

// TestTheRefusalsCommandRunsInTheRepositoryThatRefused pins the way out a
// person takes first: standing in the repository whose pinned bundle is
// missing, they run the command the refusal names, with the bare version
// of the pin, and the repository loads; the install reads no golaw.yml of
// the directory it runs in.
//
//nolint:paralleltest // the person stands in the repository, which t.Chdir sets
func TestTheRefusalsCommandRunsInTheRepositoryThatRefused(t *testing.T) {
	ta, _ := lifecycleApp(t)
	root := profiledRepository(t)
	url, _ := profiletest.BareSource(t) // the fixture is found from the module, so before the move
	t.Chdir(root)
	command := "golaw profile install " + profiletest.Source + "@" + profiletest.Version
	if _, err := ta.bind().load(t.Context(), root, now()); err == nil || !strings.HasSuffix(err.Error(), command) {
		t.Fatalf("the refusal before the install = %v, want it to end with %q", err, command)
	}
	if got := ta.run(t.Context(), []string{"profile", "install", url + "@" + profiletest.Version}); got != exitClean {
		t.Fatalf("the install in the repository: exit %d, stderr %q", got, ta.stderr.String())
	}
	if _, err := ta.bind().load(t.Context(), root, now()); err != nil {
		t.Fatalf("the repository loads after the install: %v", err)
	}
}
