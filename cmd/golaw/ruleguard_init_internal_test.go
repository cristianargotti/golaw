package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
)

// adoptedModule is the go.mod of a module that applied the dsl proposal of
// init, the state the idempotence tests of init start from.
const adoptedModule = "module example.test/service\n\ngo 1.27\n\nrequire github.com/quasilyte/go-ruleguard/dsl v0.3.23\n"

// TestInitProposesTheRuleguardDSLUntilApplied holds the one standing
// proposal of init: a module without the ruleguard dsl is offered the
// requirement on every run and its go.mod is never written, and once the
// person applies the line a further run proposes and writes nothing.
func TestInitProposesTheRuleguardDSLUntilApplied(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	const module = "module example.test/service\n\ngo 1.27\n"
	repo.Write("go.mod", module)
	ta := initBindApp(t)
	args := []string{"init", "--apply", repo.Root}
	for run := range 2 {
		ta.stdout.Reset()
		if code := ta.run(t.Context(), args); code != exitClean {
			t.Fatalf("run %d exit = %d: %s", run, code, ta.stderr.String())
		}
		out := ta.stdout.String()
		if !strings.Contains(out, "Propostas sem gravação:\n--- go.mod\n") || !strings.Contains(out, "+require github.com/quasilyte/go-ruleguard/dsl v0.3.23\n") {
			t.Fatalf("run %d does not propose the dsl: %q", run, out)
		}
		if got, err := os.ReadFile(filepath.Join(repo.Root, "go.mod")); err != nil || string(got) != module {
			t.Fatalf("run %d wrote go.mod: %q, %v", run, got, err)
		}
	}
	repo.Write("go.mod", module+"require github.com/quasilyte/go-ruleguard/dsl v0.3.23\n")
	times := managedTimes(t, repo.Root)
	ta.stdout.Reset()
	if code := ta.run(t.Context(), args); code != exitClean {
		t.Fatalf("applied exit = %d: %s", code, ta.stderr.String())
	}
	requireSilentApply(t, ta.stdout.String(), repo.Root, times)
}
