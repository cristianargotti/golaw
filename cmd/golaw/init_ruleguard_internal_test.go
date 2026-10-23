package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
)

func TestInitProposesTheRuleguardModuleAndNeverWritesGoMod(t *testing.T) {
	t.Parallel()
	const module = "module example.test/birth\n\ngo 1.27\n"
	repo := gittest.New(t)
	repo.Commit("chore: establish application", map[string]string{"go.mod": module, "main.go": "package main\n\nfunc main() {}\n"})
	ta := initBindApp(t)
	if code := ta.run(t.Context(), []string{"init", repo.Root}); code != exitClean {
		t.Fatalf("dry run exit = %d: %s", code, ta.stderr.String())
	}
	out := ta.stdout.String()
	if !strings.Contains(out, "+++ scripts/lint/rules.go") || !strings.Contains(out, "github.com/quasilyte/go-ruleguard/dsl") {
		t.Fatalf("the rules file and its module are not one reviewed diff:\n%s", out)
	}
	ta.stdout.Reset()
	if code := ta.run(t.Context(), []string{"init", "--apply", repo.Root}); code != exitClean {
		t.Fatalf("apply exit = %d: %s", code, ta.stderr.String())
	}
	if got, err := os.ReadFile(filepath.Join(repo.Root, "go.mod")); err != nil || string(got) != module {
		t.Fatalf("init wrote go.mod: %q, %v", got, err)
	}
	ta.stderr.Reset()
	if code := ta.run(t.Context(), []string{"init", "--apply", "--accept", "go.mod", repo.Root}); code != exitFailure ||
		!strings.Contains(ta.stderr.String(), "go get") {
		t.Fatalf("accepting go.mod = %d: %s", code, ta.stderr.String())
	}
}
