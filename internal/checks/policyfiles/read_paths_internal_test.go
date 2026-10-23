package policyfiles

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/proof"
)

// TestConfiguredNamesTheFilesTheReadersRead holds the prefetch predicate to
// the files the readers of the family read: a path it misses still reads
// alone, so a wrong predicate fails silently, with one more git question
// per file and no wrong verdict.
func TestConfiguredNamesTheFilesTheReadersRead(t *testing.T) {
	t.Parallel()
	match := configured("records/decisions")
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"Makefile", true},
		{"GNUmakefile", true},
		{"go.mod", true},
		{"lefthook.yml", true},
		{".golangci.yml", true},
		{".gitleaks.toml", true},
		{"sqlc.yaml", true},
		{".redocly.lint-ignore.yaml", true},
		{"CODEOWNERS", true},
		{"tools/go.mod", true},
		{"scripts/lint/rules.go", true},
		{"docs/CODEOWNERS", true},
		{".github/CODEOWNERS", true},
		{".github/dependabot.yml", true},
		{".github/dependabot.yaml", true},
		{"golaw.mk", true},
		{"build/rules.mk", true},
		{"Dockerfile", true},
		{"deploy/api.Dockerfile", true},
		{"Dockerfile.dev", true},
		{".github/workflows/ci.yml", true},
		{".github/workflows/nested/release.yaml", true},
		{"records/decisions/0001-inherited-baseline.md", true},
		{"records/decisions/0002-QUEUE.MD", true},
		{"records/decisions/notes.txt", false},
		{"docs/adr/0001-inherited-baseline.md", false},
		{"sub/Makefile", false},
		{"internal/tools/go.mod", false},
		{"sub/.golangci.yml", false},
		{"README.md", false},
		{"Dockerfile.md", false},
		{"a/a.go", false},
		{"Makefile.bak", false},
		{"go.sum", false},
		{".github/other.yml", false},
	} {
		if got := match(tc.path); got != tc.want {
			t.Errorf("configured(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

// prefetchingSnapshot counts the prefetches a check asks of its snapshot
// and keeps the predicate of the last one.
type prefetchingSnapshot struct {
	*proof.Tree

	prefetches int
	match      func(string) bool
}

func (s *prefetchingSnapshot) Prefetch(match func(string) bool) {
	s.prefetches++
	s.match = match
}

// TestEveryCheckPrefetchesBeforeItRuns holds each check of Checks to one
// prefetch of the configuration before its own reads, so the first check
// of a snapshot loads the batch whether it reads a file or selects a tree.
func TestEveryCheckPrefetchesBeforeItRuns(t *testing.T) {
	t.Parallel()
	for _, check := range Checks() {
		snapshot := &prefetchingSnapshot{Tree: proof.NewTree(nil)}
		in := makeInput(nil)
		in.Snapshot = snapshot
		if _, err := check.Run(t.Context(), in); err != nil {
			t.Fatalf("%s: %v", check.Spec().ID, err)
		}
		if snapshot.prefetches == 0 || !snapshot.match("Makefile") || snapshot.match("a/a.go") {
			t.Errorf("%s prefetched %d times, with a predicate that misses the configuration", check.Spec().ID, snapshot.prefetches)
		}
	}
}
