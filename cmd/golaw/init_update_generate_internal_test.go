package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
)

const handwrittenStore = "package postgres\n\n// Plans is written by a person.\ntype Plans struct{}\n"

// withoutGenerate is a policy as a kit before M7 wrote it: the generate
// block and its indented keys removed, the pin moved back.
func withoutGenerate(text string) string {
	var kept []string
	inside := false
	for _, line := range strings.SplitAfter(text, "\n") {
		switch {
		case strings.TrimRight(line, "\r\n") == "generate:":
			inside = true
		case inside && strings.HasPrefix(line, " "):
		default:
			inside = false
			kept = append(kept, line)
		}
	}
	return strings.Replace(strings.Join(kept, ""), testVersion, "9.9.8", 1)
}

// enrolledBeforeM7 applies init and rewrites golaw.yml as an older kit
// left it, so the update is the first run that may write the block.
func enrolledBeforeM7(t *testing.T, ta testApp, repo *gittest.Repo) string {
	t.Helper()
	if code := ta.run(t.Context(), []string{"init", "--apply", repo.Root}); code != exitClean {
		t.Fatalf("initial exit = %d: %s", code, ta.stderr.String())
	}
	path := filepath.Join(repo.Root, "golaw.yml")
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	older := withoutGenerate(string(current))
	if strings.Contains(older, "generate:") {
		t.Fatalf("the older policy still names the block:\n%s", older)
	}
	repo.Write("golaw.yml", older)
	return path
}

func updateTwice(t *testing.T, ta testApp, root string) {
	t.Helper()
	for range 2 {
		if code := ta.run(t.Context(), []string{"init", "--update", "--apply", root}); code != exitClean {
			t.Fatalf("update exit = %d: %s", code, ta.stderr.String())
		}
	}
}

func TestInitUpdateWritesTheDetectedGenerateBlockOnce(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	repo.Write("go.mod", adoptedModule)
	repo.Write("internal/adapters/outbound/postgres/plans.go", handwrittenStore)
	ta := initBindApp(t)
	path := enrolledBeforeM7(t, ta, repo)
	updateTwice(t, ta, repo.Root)
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "adapter_sql: internal/adapters/outbound/{package}"
	if strings.Count(string(updated), "generate:") != 1 || strings.Count(string(updated), want) != 1 {
		t.Fatalf("golaw.yml lacks the detected generate block once:\n%s", updated)
	}
}

// The near row: a reviewed block is kept as written and no detection runs,
// so two layouts the person already settled never fail the update.
func TestInitUpdateKeepsAReviewedGenerateBlock(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	repo.Write("go.mod", adoptedModule)
	ta := initBindApp(t)
	if code := ta.run(t.Context(), []string{"init", "--apply", repo.Root}); code != exitClean {
		t.Fatalf("initial exit = %d: %s", code, ta.stderr.String())
	}
	path := filepath.Join(repo.Root, "golaw.yml")
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	reviewed := strings.Replace(string(current), "  support: internal/gensupport", "  support: pkg/gensupport", 1)
	if reviewed == string(current) {
		t.Fatalf("init wrote no support line to review:\n%s", current)
	}
	repo.Write("golaw.yml", strings.Replace(reviewed, testVersion, "9.9.8", 1))
	repo.Write("internal/adapters/out/postgres/plans.go", handwrittenStore)
	repo.Write("internal/adapters/outbound/meta/client.go", strings.ReplaceAll(handwrittenStore, "postgres", "meta"))
	updateTwice(t, ta, repo.Root)
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(updated), "generate:") != 1 || !strings.Contains(string(updated), "  support: pkg/gensupport") {
		t.Fatalf("the reviewed generate block changed:\n%s", updated)
	}
}
