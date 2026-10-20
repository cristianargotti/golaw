package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/buildinfo"
	"github.com/cristianargotti/golaw/internal/checks"
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// stubCheck is a file check with a fixed result, so a test drives the
// decision the scan command maps to an exit code.
type stubCheck struct {
	result model.Result
}

func (stubCheck) Spec() model.Spec {
	return model.Spec{ID: "stub.rule", Rule: "rules/0@1", Scope: model.ScopeFile, Owner: model.OwnerCore, Proof: "stub.rule"}
}

func (c stubCheck) Run(context.Context, model.Input) (model.Result, error) { return c.result, nil }

// scanApp wires the scan seams with fakes: one stub check, one file, the
// proof policy and a renderer that prints the decision in one line.
func scanApp(t *testing.T, result model.Result) testApp {
	t.Helper()
	ta := newTestApp(t)
	ta.registry = func() (*engine.Registry, error) {
		b := engine.New()
		if err := b.Register(stubCheck{result: result}); err != nil {
			t.Fatal(err)
		}
		return b.Build()
	}
	ta.snapshot = func(_ context.Context, _ string) (model.Snapshot, error) {
		return proof.NewTree(map[string][]byte{"docs/a.md": []byte("title\n")}), nil
	}
	ta.load = func(_ context.Context, _ string, _ time.Time) (*model.EffectivePolicy, error) {
		return proof.Policy(), nil
	}
	ta.render = func(w io.Writer, in model.Input, _ *engine.Run, d engine.Decision, asJSON bool) error {
		_, err := fmt.Fprintf(w, "%s findings=%d unmeasured=%d json=%v digest=%s\n",
			d.Verdict, len(d.Findings), len(d.Unmeasured), asJSON, in.Policy.Digest)
		if err != nil {
			return fmt.Errorf("render: %w", err)
		}
		return nil
	}
	return ta
}

func finding() model.Result {
	return model.Result{Status: model.Measured(), Findings: []model.Finding{
		{Path: "docs/a.md", Line: 1, Code: "em-dash", Message: "em dash in text", Severity: model.SeverityDeny},
	}}
}

func TestScanExitCodes(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		result model.Result
		code   int
		line   string
	}{
		"clean":      {model.Result{Status: model.Measured()}, exitClean, "clean findings=0 unmeasured=0"},
		"findings":   {finding(), exitFindings, "fail findings=1 unmeasured=0"},
		"unmeasured": {model.Result{Status: model.Unmeasured("no reader")}, exitFailure, "fail findings=0 unmeasured=1"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ta := scanApp(t, tc.result)
			if got := ta.run(t.Context(), []string{"scan", "."}); got != tc.code {
				t.Fatalf("exit = %d, want %d, stderr = %q", got, tc.code, ta.stderr.String())
			}
			if !strings.HasPrefix(ta.stdout.String(), tc.line) {
				t.Fatalf("stdout = %q, want prefix %q", ta.stdout.String(), tc.line)
			}
			if ta.stderr.Len() != 0 {
				t.Fatalf("stderr = %q", ta.stderr.String())
			}
		})
	}
}

func TestScanPassesTheJSONFlagAndTheDefaultPath(t *testing.T) {
	t.Parallel()
	ta := scanApp(t, model.Result{Status: model.Measured()})
	var root string
	ta.snapshot = func(_ context.Context, path string) (model.Snapshot, error) {
		root = path
		return proof.NewTree(map[string][]byte{"docs/a.md": []byte("title\n")}), nil
	}
	if got := ta.run(t.Context(), []string{"scan", "--json"}); got != exitClean {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	if root != "." || !strings.Contains(ta.stdout.String(), "json=true digest=proof") {
		t.Fatalf("root = %q, stdout = %q", root, ta.stdout.String())
	}
}

func TestScanReportsEverySeamFailureAsExitTwo(t *testing.T) {
	t.Parallel()
	broken := map[string]func(ta *testApp){
		"load": func(ta *testApp) {
			ta.load = func(context.Context, string, time.Time) (*model.EffectivePolicy, error) { return nil, errBoom }
		},
		"snapshot": func(ta *testApp) {
			ta.snapshot = func(context.Context, string) (model.Snapshot, error) { return nil, errBoom }
		},
		"registry": func(ta *testApp) {
			ta.registry = func() (*engine.Registry, error) { return nil, errBoom }
		},
		"render": func(ta *testApp) {
			ta.render = func(io.Writer, model.Input, *engine.Run, engine.Decision, bool) error { return errBoom }
		},
	}
	for name, breakSeam := range broken {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ta := scanApp(t, model.Result{Status: model.Measured()})
			breakSeam(&ta)
			if got := ta.run(t.Context(), []string{"scan"}); got != exitFailure {
				t.Fatalf("exit = %d", got)
			}
			if got := ta.stderr.String(); got != "golaw: scan: boom\n" {
				t.Fatalf("stderr = %q", got)
			}
		})
	}
}

// boundApp runs init and scan over the production seams of a test binding,
// so the policy, source and report packages meet a real repository under a
// temporary home.
func boundApp(t *testing.T) testApp {
	t.Helper()
	b, _ := testBinding(t)
	ta := newTestApp(t)
	ta.bind = func() binding { return b }
	ta.buildInfo = func() buildinfo.Info { return b.info }
	ta.registry = func() (*engine.Registry, error) { return checks.Registry() }
	ta.snapshot, ta.load, ta.render = b.snapshot, b.load, b.render
	ta.compile, ta.enroll, ta.committed = b.compile, b.enroll, b.committed
	return ta
}

// enrolledByInit is an unborn repository that init --apply enrolled over
// the seams of ta, with the output of init cleared.
func enrolledByInit(t *testing.T, ta testApp) string {
	t.Helper()
	root := repository(t, false)
	if got := ta.run(t.Context(), []string{"init", "--apply", root}); got != exitClean {
		t.Fatalf("init exit = %d, stderr = %q", got, ta.stderr.String())
	}
	ta.stdout.Reset()
	return root
}

func TestScanRefusesAnEnrolledCheckoutWithoutItsPolicy(t *testing.T) {
	t.Parallel()
	control := boundApp(t)
	root := enrolledByInit(t, control)
	if got := control.run(t.Context(), []string{"scan", root}); got != exitClean {
		t.Fatalf("unchanged exit = %d, stdout = %q, stderr = %q", got, control.stdout.String(), control.stderr.String())
	}
	deleted := boundApp(t)
	root = enrolledByInit(t, deleted)
	if err := os.Remove(filepath.Join(root, "golaw.yml")); err != nil {
		t.Fatal(err)
	}
	if got := deleted.run(t.Context(), []string{"scan", root}); got != exitFailure || !strings.Contains(deleted.stderr.String(), "golaw.yml is gone from the checkout") {
		t.Fatalf("deleted exit = %d, stdout = %q, stderr = %q", got, deleted.stdout.String(), deleted.stderr.String())
	}
}

func TestCodeOfTreatsDenyLikeFail(t *testing.T) {
	t.Parallel()
	if got := codeOf(engine.Decision{Verdict: engine.VerdictDeny}); got != exitFindings {
		t.Fatalf("deny = %d", got)
	}
}

// notesRepository is an enrolled repository whose commit holds a README.md
// with a dash, an ignore file hiding a generated tree and a note the
// checkout has deleted since.
func notesRepository(t *testing.T, ta testApp) string {
	t.Helper()
	root := enrolledByInit(t, ta)
	for name, content := range map[string]string{"README.md": "one \u2014 two\n", ".gitignore": "gen/\n", "gone.md": "gone\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "docs: add the notes")
	if err := os.Remove(filepath.Join(root, "gone.md")); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestScanLeavesAnUnreadableFileUnmeasured is the scan of section 5.2 over
// the real classification of the source: a README.md holding a dash and an
// ignore file hiding a generated tree, both unreadable, answer Unmeasured
// with exit 2 instead of clean, while a tracked file deleted from the
// checkout is a deletion the next commit carries, never unreadable.
func TestScanLeavesAnUnreadableFileUnmeasured(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" || os.Getuid() == 0 {
		t.Skip("permission bits do not refuse the owner here")
	}
	ta := boundApp(t)
	root := notesRepository(t, ta)
	for _, tc := range []struct{ name, want string }{
		{"README.md", "text.dashes unmeasured read README.md: path unreadable"},
		{".gitignore", "paths.generated-tracked unmeasured read .gitignore: path unreadable"},
	} {
		name, want := tc.name, tc.want
		path := filepath.Join(root, name)
		if err := os.Chmod(path, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := os.Chmod(path, 0o600); err != nil {
				t.Error(err)
			}
		})
		ta.stdout.Reset()
		if got := ta.run(t.Context(), []string{"scan", root}); got != exitFailure {
			t.Fatalf("%s: exit = %d, stdout = %q, stderr = %q", name, got, ta.stdout.String(), ta.stderr.String())
		}
		if out := ta.stdout.String(); !strings.Contains(out, want) || strings.Contains(out, "gone.md") {
			t.Errorf("%s: missing %q, or a deleted file judged, in %q", name, want, out)
		}
	}
}
