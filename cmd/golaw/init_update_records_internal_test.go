package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
)

// TestInitUpdateKeepsTheFirstUpgradeOfAnEarlierBaselineClean holds the
// first update of an adoption whose baseline record predates the measured
// tools module: the update proposes the modules the kit's tools/go.mod
// requires, scan names them until the person applies that proposal, and the
// applied proposal leaves policy.dependency-records clean.
func TestInitUpdateKeepsTheFirstUpgradeOfAnEarlierBaselineClean(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	repo.Write("go.mod", adoptedModule)
	ta := initBindApp(t)
	olderPolicy(t, ta, repo, filepath.Join(repo.Root, "golaw.yml"))
	record := filepath.Join(repo.Root, "docs", "adr", "0001-inherited-baseline.md")
	fresh, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	earlier := string(fresh)
	for _, module := range []string{"github.com/evilmartians/lefthook/v2", "github.com/golangci/golangci-lint/v2", "github.com/zricethezav/gitleaks/v8", "golang.org/x/vuln"} {
		earlier = strings.Replace(earlier, "  - "+module+"\n", "", 1)
	}
	repo.Write("docs/adr/0001-inherited-baseline.md", earlier)
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "feat: adopt the golaw policy")
	ta.stdout.Reset()
	if code := ta.run(t.Context(), []string{"init", "--update", "--apply", repo.Root}); code != exitClean ||
		!strings.Contains(ta.stdout.String(), "+  - github.com/evilmartians/lefthook/v2\n") {
		t.Fatalf("update exit = %d: %s%s", code, ta.stdout.String(), ta.stderr.String())
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "feat: update the golaw kit")
	red := boundApp(t)
	red.run(t.Context(), []string{"scan", repo.Root})
	if !strings.Contains(red.stdout.String(), "tools/go.mod:") || !strings.Contains(red.stdout.String(), "policy.dependency-records") {
		t.Fatalf("scan before the proposal = %q", red.stdout.String())
	}
	repo.Write("docs/adr/0001-inherited-baseline.md", string(fresh))
	repo.Git("commit", "-q", "-am", "docs: record the kit tools")
	near := boundApp(t)
	near.run(t.Context(), []string{"scan", repo.Root})
	if strings.Contains(near.stdout.String(), "policy.dependency-records") {
		t.Fatalf("scan after the proposal = %q", near.stdout.String())
	}
}
