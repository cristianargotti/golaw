package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks"
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/report"
)

// stagedApp is the test app over the production binding of a temporary
// home, with the catalog and the report of that binding.
func stagedApp(t *testing.T) testApp {
	t.Helper()
	ta := bindApp(t)
	ta.registry = func() (*engine.Registry, error) { return checks.Registry() }
	ta.render = ta.bind().render
	return ta
}

// stagedDash is the dash the dash law refuses, spelled by code point so this
// file holds none.
func stagedDash() string { return string(rune(0x2014)) }

// stageFiles writes files under root and stages them.
func stageFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	writeFiles(t, root, files)
	for name := range files {
		gitRun(t, root, "add", "--", name)
	}
}

func TestScanStagedReportsOnlyWhatTheIndexIntroduces(t *testing.T) {
	t.Parallel()
	ta := stagedApp(t)
	root := repository(t, true)
	stageFiles(t, root, map[string]string{"notes.md": "a " + stagedDash() + " b\n"})
	writeFiles(t, root, map[string]string{"draft.md": "c " + stagedDash() + " d\n"})
	if got := ta.run(t.Context(), []string{"scan", "--staged", root}); got != exitFindings {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", got, ta.stdout.String(), ta.stderr.String())
	}
	out := ta.stdout.String()
	if !strings.Contains(out, "notes.md:1:3 deny text.dashes") || strings.Contains(out, "draft.md") || !strings.Contains(out, "verdict fail: 1 findings") {
		t.Errorf("stdout = %q", out)
	}
	clean := stagedApp(t)
	cleanRoot := repository(t, true)
	writeFiles(t, cleanRoot, map[string]string{"draft.md": "c " + stagedDash() + " d\n"})
	if got := clean.run(t.Context(), []string{"scan", "--staged", cleanRoot}); got != exitClean || !strings.Contains(clean.stdout.String(), "verdict clean") {
		t.Errorf("a dash only in the worktree: exit %d, stdout %q", got, clean.stdout.String())
	}
}

func TestScanStagedPrintsTheIndexInJSON(t *testing.T) {
	t.Parallel()
	ta := stagedApp(t)
	root := repository(t, true)
	stageFiles(t, root, map[string]string{"notes.md": "clean\n"})
	if got := ta.run(t.Context(), []string{"scan", "--staged", "--json", root}); got != exitClean {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	var doc report.Document
	if err := json.Unmarshal(ta.stdout.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Mode != "staged" || !strings.HasPrefix(doc.Snapshot, "index:") || doc.Verdict != "clean" {
		t.Errorf("document = %+v", doc)
	}
}

func TestScanStagedRefusesAStagedWeakening(t *testing.T) {
	t.Parallel()
	ta := stagedApp(t)
	root := repository(t, true)
	stageFiles(t, root, map[string]string{"golaw.yml": strings.Replace(string(renderedPolicy(t)), "file_lines: 300", "file_lines: 301", 1)})
	if got := ta.run(t.Context(), []string{"scan", "--staged", root}); got != exitFailure {
		t.Fatalf("exit = %d, stdout = %q", got, ta.stdout.String())
	}
	if got := ta.stderr.String(); !strings.Contains(got, "golaw: scan: ") || !strings.Contains(got, "staged golaw.yml") || !strings.Contains(got, "git restore --staged golaw.yml") {
		t.Errorf("stderr = %q", got)
	}
}

func TestScanStagedReportsEveryFailureAsExitTwo(t *testing.T) {
	t.Parallel()
	outside := stagedApp(t)
	if got := outside.run(t.Context(), []string{"scan", "--staged", t.TempDir()}); got != exitFailure || !strings.Contains(outside.stderr.String(), "golaw: scan: ") {
		t.Errorf("outside git: exit %d, stderr %q", got, outside.stderr.String())
	}
	broken := stagedApp(t)
	broken.registry = func() (*engine.Registry, error) { return nil, errBoom }
	if got := broken.run(t.Context(), []string{"scan", "--staged", repository(t, true)}); got != exitFailure || broken.stderr.String() != "golaw: scan: boom\n" {
		t.Errorf("registry: exit %d, stderr %q", got, broken.stderr.String())
	}
}

// Before the adoption commit of a repository with a history, HEAD holds no
// golaw.yml while the index holds the one init --apply enrolled: the next
// commit records the index, so its policy is the one in force.
func TestScanStagedTakesTheEnrolledPolicyBeforeTheAdoptionCommit(t *testing.T) {
	t.Parallel()
	ta := stagedApp(t)
	root := repository(t, false)
	gitRun(t, root, "add", "go.mod")
	gitRun(t, root, "commit", "-q", "-m", "build: start the module")
	b := ta.bind()
	data := renderedPolicy(t)
	p, err := b.compile(data, now())
	if err != nil {
		t.Fatal(err)
	}
	if err := b.enroll(root, data, p, now()); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", "golaw.yml")
	if got := ta.run(t.Context(), []string{"scan", "--staged", root}); got != exitClean || !strings.Contains(ta.stdout.String(), "verdict clean") {
		t.Errorf("the adoption commit staged: exit %d, stdout %q, stderr %q", got, ta.stdout.String(), ta.stderr.String())
	}
}
