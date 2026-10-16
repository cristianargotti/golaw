package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/buildinfo"
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// testVersion is the version the test binding carries and the policy pins.
const testVersion = "9.9.9"

// gitRun runs one git command in a directory and fails the test on error.
func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// renderedPolicy is the policy file init renders for a Go module pinned
// at the test version.
func renderedPolicy(t *testing.T) []byte {
	t.Helper()
	in, err := bootstrap.Inputs{
		KitVersion: testVersion, Language: model.DefaultLanguage,
		Layout: bootstrap.Layout{Main: bootstrap.MainRoot, Decisions: "docs/adr", Go: true},
	}.Normalize()
	if err != nil {
		t.Fatal(err)
	}
	data, err := bootstrap.Render(in)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// repository creates a git repository with a go.mod and the rendered policy,
// committed at HEAD when commit is true, and answers its real path.
func repository(t *testing.T, commit bool) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "init", "-q")
	gitRun(t, root, "config", "user.email", "tester@example.test")
	gitRun(t, root, "config", "user.name", "tester")
	for name, content := range map[string][]byte{
		"go.mod":    []byte("module example.test/m\n\ngo 1.27\n"),
		"golaw.yml": renderedPolicy(t),
	} {
		if err := os.WriteFile(filepath.Join(root, name), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if commit {
		gitRun(t, root, "add", "-A")
		gitRun(t, root, "commit", "-q", "-m", "build: the policy")
	}
	return root
}

// testBinding binds the production adapters to a temporary home: the state
// and configuration directories live under it and nothing reads the
// process environment.
func testBinding(t *testing.T) (binding, string) {
	t.Helper()
	home := t.TempDir()
	values := map[string]string{
		"HOME": home, "XDG_STATE_HOME": filepath.Join(home, "state"), "XDG_CONFIG_HOME": filepath.Join(home, "config"),
	}
	b := binding{
		goos: "linux",
		env:  func(name string) string { return values[name] },
		home: func() (string, error) { return home, nil },
		info: buildinfo.Info{Version: testVersion, Module: "example.test/kit", Stamped: true},
	}
	return b, home
}

func now() time.Time { return time.Date(2026, time.October, 14, 12, 0, 0, 0, time.UTC) }

func TestBindingLoadsThePolicyAtHead(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	root := repository(t, true)
	p, err := b.load(t.Context(), root, now())
	if err != nil {
		t.Fatal(err)
	}
	if p.KitVersion != testVersion || p.Enrollment.State != model.EnrollmentNone || !p.Has("go") {
		t.Fatalf("policy = %+v", p)
	}
	if got, err := b.discover(t.Context(), filepath.Join(root, "docs")); err == nil || got != "" {
		t.Fatalf("discover of an absent directory = %q, %v", got, err)
	}
	if got, err := b.discover(t.Context(), root); err != nil || got != root {
		t.Fatalf("discover = %q, %v", got, err)
	}
}

func TestBindingChecksThePinOfReleasedBuildsOnly(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	root := repository(t, true)
	b.info.Version = "1.0.0"
	if _, err := b.load(t.Context(), root, now()); !errors.Is(err, model.ErrPolicy) {
		t.Fatalf("a mismatching pin was accepted: %v", err)
	}
	for name, info := range map[string]buildinfo.Info{
		"development": {Version: buildinfo.DevelVersion},
		"untagged":    {Version: "0.0.0-20261015004959-dd849024b474", Stamped: true},
	} {
		b.info = info
		if _, err := b.load(t.Context(), root, now()); err != nil {
			t.Fatalf("a %s build was refused: %v", name, err)
		}
		if b.pin() != "" {
			t.Fatalf("a %s build carries a pin", name)
		}
	}
}

func TestBindingCompilesEnrollsAndExplains(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	root := repository(t, false)
	data := renderedPolicy(t)
	p, err := b.compile(data, now())
	if err != nil {
		t.Fatal(err)
	}
	if _, invalid := b.compile([]byte("golaw: 2\n"), now()); invalid == nil {
		t.Fatal("an invalid policy compiled")
	}
	if err = b.enroll(root, data, p, now()); err != nil {
		t.Fatal(err)
	}
	loaded, err := b.loaded(t.Context(), root, now())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Origin != policy.OriginBootstrap || loaded.Policy.Enrollment.State != model.EnrollmentEnrolled || loaded.Policy.Digest != p.Digest {
		t.Fatalf("loaded = %+v", loaded)
	}
	text, err := b.explain(t.Context(), root, now())
	if err != nil || !strings.Contains(text, "kit.version") || !strings.Contains(text, testVersion) {
		t.Fatalf("explain = %q, %v", text, err)
	}
	if err := b.enroll(root, []byte("golaw: [\n"), p, now()); err == nil {
		t.Fatal("undecodable bytes were enrolled")
	}
}

func TestBindingSnapshotAndRender(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	root := repository(t, true)
	snapshot, err := b.snapshot(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, len(snapshot.Files()))
	for _, f := range snapshot.Files() {
		paths = append(paths, f.Path)
	}
	if !strings.Contains(strings.Join(paths, " "), "go.mod") {
		t.Fatalf("files = %v", paths)
	}
	p, err := b.load(t.Context(), root, now())
	if err != nil {
		t.Fatal(err)
	}
	in := model.Input{Mode: model.ModeScan, Policy: p, Snapshot: snapshot}
	decision := engine.Decision{Mode: model.ModeScan, Verdict: engine.VerdictClean}
	var text, document bytes.Buffer
	if err = b.render(&text, in, nil, decision, false); err != nil || !strings.Contains(text.String(), "verdict clean") {
		t.Fatalf("text = %q, %v", text.String(), err)
	}
	named, ok := snapshot.(identified)
	if !ok {
		t.Fatal("the worktree has no identity")
	}
	identity := named.Identity()
	if err = b.render(&document, in, nil, decision, true); err != nil || !strings.Contains(document.String(), identity) {
		t.Fatalf("json = %q, %v", document.String(), err)
	}
	if _, err = b.snapshot(t.Context(), t.TempDir()); err == nil {
		t.Fatal("a directory outside git was frozen")
	}
}

func TestInstallStateIsTheStateDirectoryOfTheRecord(t *testing.T) {
	t.Parallel()
	b, home := testBinding(t)
	state, err := b.installState()
	dirs, dirsErr := b.dirs()
	if err != nil || dirsErr != nil || state != filepath.Join(home, "state", "golaw") || state != dirs.State {
		t.Fatalf("state = %q, %v; record = %q, %v", state, err, dirs.State, dirsErr)
	}
	b.goos = "darwin"
	if darwin, darwinErr := b.installState(); darwinErr != nil || darwin != state {
		t.Fatalf("darwin state = %q, %v", darwin, darwinErr)
	}
	b.env = func(string) string { return "" }
	if _, err := b.installState(); err == nil {
		t.Fatal("the state was resolved without a home")
	}
}

func TestBindingReportsAnUnresolvableHome(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	b.env = func(string) string { return "" }
	root := repository(t, true)
	if _, err := b.load(t.Context(), root, now()); err == nil {
		t.Fatal("the policy loaded without a state directory")
	}
	if err := b.enroll(root, renderedPolicy(t), &model.EffectivePolicy{}, now()); err == nil {
		t.Fatal("the record was written without a state directory")
	}
}
