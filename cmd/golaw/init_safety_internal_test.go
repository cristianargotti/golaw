package main

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

func TestInitRefusesReenrollmentBeforeWritingTemplates(t *testing.T) {
	t.Parallel()
	for _, apply := range []bool{true, false} {
		t.Run(map[bool]string{true: "apply", false: "preview"}[apply], func(t *testing.T) {
			t.Parallel()
			requireReenrollmentRefused(t, apply)
		})
	}
}

// requireReenrollmentRefused requires init --apply to refuse a weakening
// before any output or write, and the preview to print the diff and name
// the refusal --apply gives, as M1 printed its diff.
func requireReenrollmentRefused(t *testing.T, apply bool) {
	t.Helper()
	repo := gittest.New(t)
	ta := initBindApp(t)
	if code := ta.run(t.Context(), []string{"init", "--apply", repo.Root}); code != exitClean {
		t.Fatal(ta.stderr.String())
	}
	data, err := bootstrap.Current(repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	config, err := policy.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	prior := strings.Replace(string(data), "file_lines: 300", "file_lines: 200", 1)
	strict, err := policy.Decode([]byte(prior))
	if err != nil {
		t.Fatal(err)
	}
	dirs, err := ta.bind().dirs()
	if err != nil {
		t.Fatal(err)
	}
	record := policy.Record{
		Root: repo.Root, PolicyDigest: policy.Digest(strict), KitVersion: config.Kit.Version,
		Bootstrap: prior, WrittenBy: policy.WrittenByInit, WrittenAt: now(),
	}
	if writeErr := policy.WriteRecord(dirs.State, record); writeErr != nil {
		t.Fatal(writeErr)
	}
	repo.Write("go.mod", "module example.test/service\n\ngo 1.27\n")
	ta.stdout.Reset()
	if apply {
		requireWeakeningRefused(t, ta, repo.Root)
	} else {
		requireWeakeningPreviewed(t, ta, repo.Root)
	}
	requireNoInitTemplates(t, repo.Root)
	after, err := bootstrap.Current(repo.Root)
	if err != nil || !bytes.Equal(after, data) {
		t.Fatalf("policy changed: %q, %v", after, err)
	}
}

func requireWeakeningRefused(t *testing.T, ta testApp, root string) {
	t.Helper()
	if code := ta.run(t.Context(), []string{"init", "--apply", root}); code != exitFailure {
		t.Fatalf("weakening exit = %d", code)
	}
	if !strings.Contains(ta.stderr.String(), "weakens") || ta.stdout.Len() != 0 {
		t.Fatalf("refusal: %q, %q", ta.stdout.String(), ta.stderr.String())
	}
}

func requireWeakeningPreviewed(t *testing.T, ta testApp, root string) {
	t.Helper()
	if code := ta.run(t.Context(), []string{"init", root}); code != exitClean {
		t.Fatalf("preview of a weakening exit = %d: %s", code, ta.stderr.String())
	}
	out := strings.TrimSuffix(ta.stdout.String(), "\n")
	last := out[strings.LastIndex(out, "\n")+1:]
	if !strings.Contains(out, "+++ lefthook.yml\n") || !strings.Contains(last, "weakens") ||
		!strings.HasPrefix(last, "init: nothing written; --apply would refuse: ") {
		t.Fatalf("preview of a weakening = %q", ta.stdout.String())
	}
}

func TestInitReviewedSettingsShapeTemplatesWithoutDefaultingPolicy(t *testing.T) {
	t.Parallel()
	ta, record := initApp(t)
	root := checkout(t)
	reviewed := "golaw: 1\nkit: { version: 9.9.9 }\nlanguage: en\nlayout: { main: cmd, decisions: docs/decisions }\n"
	if err := os.WriteFile(filepath.Join(root, policy.FileName), []byte(reviewed), 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"init", "--apply", "--language", "pt-BR", root}
	if code := ta.run(t.Context(), args); code != exitClean {
		t.Fatal(ta.stderr.String())
	}
	if !strings.HasPrefix(string(record.data), reviewed) || !strings.Contains(ta.stdout.String(), "Writes:\n") {
		t.Fatalf("reviewed settings lost: %q, %q", record.data, ta.stdout.String())
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "decisions", "0001-inherited-baseline.md")); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(record.data), "capabilities:") {
		t.Fatalf("absent defaults persisted: %q", record.data)
	}
}

type initOutputAction struct {
	t      *testing.T
	root   string
	action string
	done   bool
}

func (w *initOutputAction) Write(data []byte) (int, error) {
	if !w.done && bytes.Contains(data, []byte("+++ lefthook.yml")) {
		w.done = true
		if w.action == "fail" {
			return 0, io.ErrClosedPipe
		}
		path, content := policy.FileName, "# concurrent edit\n"
		if w.action == "module" {
			path, content = "go.mod", "module example.test/changed\n\ngo 1.27\n"
		}
		if err := os.WriteFile(filepath.Join(w.root, path), []byte(content), 0o600); err != nil {
			w.t.Fatal(err)
		}
	}
	return len(data), nil
}

func TestInitRechecksPolicyAndCompletesOutputBeforeMutation(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"fail", "edit", "module"} {
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			ta, record := initApp(t)
			root := checkout(t)
			writer := &initOutputAction{t: t, root: root, action: action}
			ta.app.stdout = writer
			if code := ta.run(t.Context(), []string{"init", "--apply", root}); code != exitFailure {
				t.Fatalf("exit = %d", code)
			}
			if !writer.done || record.calls != 0 {
				t.Fatalf("output action = %v, enrollment calls = %d", writer.done, record.calls)
			}
			requireNoInitTemplates(t, root)
		})
	}
}

func TestInitPreservesAnEditDuringInitialCompilation(t *testing.T) {
	t.Parallel()
	for _, initial := range []string{"", "golaw: 1\nkit: { version: 9.9.9 }\n"} {
		t.Run(initial, func(t *testing.T) {
			t.Parallel()
			requireCompilationEditPreserved(t, initial)
		})
	}
}

func requireCompilationEditPreserved(t *testing.T, initial string) {
	t.Helper()
	ta, record := initApp(t)
	root := checkout(t)
	path := filepath.Join(root, policy.FileName)
	if initial != "" {
		if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	changed := "golaw: 1\nkit: { version: 9.9.9 }\n# concurrent review\n"
	compile := ta.compile
	calls := 0
	ta.compile = func(data []byte, when time.Time) (*model.EffectivePolicy, error) {
		calls++
		if calls == 1 {
			if err := os.WriteFile(path, []byte(changed), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		return compile(data, when)
	}
	if code := ta.run(t.Context(), []string{"init", "--apply", root}); code != exitFailure {
		t.Fatalf("concurrent policy accepted: %d", code)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != changed || record.calls != 0 {
		t.Fatalf("concurrent review lost: %q, %v, %d", after, err, record.calls)
	}
	requireNoInitTemplates(t, root)
}

func requireNoInitTemplates(t *testing.T, root string) {
	t.Helper()
	for _, path := range []string{".editorconfig", ".golangci.yml", "lefthook.yml", "golaw.mk", "Makefile"} {
		if _, err := os.Stat(filepath.Join(root, path)); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("unexpected template %s: %v", path, err)
		}
	}
}

func TestInitRefusesChangedPolicyBeforeApplication(t *testing.T) {
	t.Parallel()
	root := checkout(t)
	plan := initPlan{templates: bootstrap.TemplatePlan{
		Changes: []install.Change{{Path: "new.txt", After: []byte("new\n")}},
	}}
	if err := os.WriteFile(filepath.Join(root, policy.FileName), []byte("changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := applyInit(root, plan, io.Discard); !errors.Is(err, install.ErrChanged) {
		t.Fatalf("changed policy accepted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "new.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("template written: %v", err)
	}
}
