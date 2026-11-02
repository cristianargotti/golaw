package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/runnertest"
)

func TestGenCheckIsCleanWithoutAManifest(t *testing.T) {
	t.Parallel()
	ta, _ := genApp(t)
	if code := ta.run(t.Context(), []string{"gen", "--check"}); code != exitClean || strings.TrimSpace(ta.stdout.String()) != "golaw gen --check: clean, 0 drift, 0 open holes" {
		t.Errorf("exit = %d, stdout %q", code, ta.stdout.String())
	}
}

func TestGenCheckFailsOnTheOpenHolesOfANewKind(t *testing.T) {
	t.Parallel()
	ta, _ := newPort(t)
	if code := ta.run(t.Context(), []string{"gen", "--check"}); code != exitFindings {
		t.Fatalf("exit = %d: %s", code, ta.stdout.String())
	}
	out := ta.stdout.String()
	if !strings.Contains(out, "golaw gen --check: hole open ") || !strings.Contains(out, ": fill it and its red test") || strings.Contains(out, "golaw gen --check: drift ") {
		t.Errorf("stdout = %s", out)
	}
	if !strings.Contains(ta.stderr.String(), "lacuna aberta") {
		t.Errorf("the person's line is missing: %q", ta.stderr.String())
	}
}

// The red rows of drift: a hand edit and a missing regenerable file, each
// named with the command that fixes it; golaw gen then repairs both.
func TestGenCheckNamesDriftAndGenRepairsIt(t *testing.T) {
	t.Parallel()
	ta, root := newPort(t)
	fake := filepath.Join(root, "internal", "application", "growth", "ledgertest", "zz_generated_fake.go")
	data, err := os.ReadFile(fake)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fake, append(data, "// a hand edit\n"...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "internal", "gensupport", "zz_generated_hole.go")); err != nil {
		t.Fatal(err)
	}
	if code := ta.run(t.Context(), []string{"gen", "--check"}); code != exitFindings {
		t.Fatalf("exit = %d", code)
	}
	for _, want := range []string{
		"golaw gen --check: drift internal/application/growth/ledgertest/zz_generated_fake.go differs from its rendering: run golaw gen",
		"golaw gen --check: drift internal/gensupport/zz_generated_hole.go regenerable file missing: run golaw gen",
	} {
		if !strings.Contains(ta.stdout.String(), want) {
			t.Errorf("stdout lacks %q:\n%s", want, ta.stdout.String())
		}
	}
	if code := ta.run(t.Context(), []string{"gen"}); code != exitClean {
		t.Fatalf("gen exit = %d: %s", code, ta.stderr.String())
	}
	ta.stdout.Reset()
	ta.run(t.Context(), []string{"gen", "--check"})
	if strings.Contains(ta.stdout.String(), "drift internal") {
		t.Errorf("drift after golaw gen:\n%s", ta.stdout.String())
	}
}

func TestGenCheckWritesTheJSONReport(t *testing.T) {
	t.Parallel()
	ta, _ := newPort(t)
	ta.run(t.Context(), []string{"gen", "--check", "--json"})
	var doc struct {
		Drift []struct {
			Path  string `json:"path"`
			Cause string `json:"cause"`
			Fix   string `json:"fix"`
		} `json:"drift"`
		Holes []holeJSON `json:"holes"`
	}
	if err := json.Unmarshal(ta.stdout.Bytes(), &doc); err != nil || len(doc.Holes) == 0 {
		t.Errorf("JSON = %+v, %v: %s", doc, err, ta.stdout.String())
	}
}

// The red row of the unmeasured state: a generator the tools module
// cannot provide is exit 2, never a pass.
func TestGenCheckIsUnmeasuredWhenAGeneratorIsMissing(t *testing.T) {
	t.Parallel()
	ta, root := newPort(t)
	bound := ta.bind()
	bin := t.TempDir()
	runnertest.Install(t, bin, "go", runnertest.Behavior{Stderr: "go: no tool sqlc", Code: 1})
	bound.environ = func() []string { return []string{"PATH=" + bin} }
	ta.bind = func() binding { return bound }
	writeRepoFile(t, root, "sqlc.yaml", "version: \"2\"\n")
	writeRepoFile(t, root, "tools/go.mod", "module example.com/slice/tools\n\ngo 1.27\n")
	if code := ta.run(t.Context(), []string{"gen", "--check"}); code != exitFailure || !strings.Contains(ta.stdout.String(), "golaw gen --check: unmeasured sqlc") {
		t.Errorf("exit = %d, stdout %q", code, ta.stdout.String())
	}
}
