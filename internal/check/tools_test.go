package check_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/check"
)

func TestToolOutputs(t *testing.T) {
	t.Parallel()
	vet := "# example.test/m\nvet: ./a.go:6:2: unreachable code\n./b.go:1:1: printf mismatch\nnote: elsewhere\n"
	diff := "diff a.go.orig a.go\n--- a.go.orig\n+++ a.go\n@@ -1 +1 @@\n-x\n+y\n+++ b/sub/b.go\t2026\n"
	for _, tc := range []struct {
		name string
		got  []string
		want []string
	}{
		{"red-located-lines-of-vet", check.Located(vet), []string{"a.go:6:2: unreachable code", "b.go:1:1: printf mismatch"}},
		{"near-no-located-line", check.Located("level=error msg=config\n"), nil},
		{"red-unformatted-files", check.Unformatted(diff), []string{
			"a.go: not formatted, run golangci-lint fmt", "sub/b.go: not formatted, run golangci-lint fmt",
		}},
		{"near-an-empty-diff", check.Unformatted(""), nil},
		{"red-failed-keeps-the-located-lines", check.Failed("lint", 1, []byte("a.go:1:1: x (errcheck)\n"), []byte("level=warning\n")), []string{"a.go:1:1: x (errcheck)"}},
		{"red-failed-falls-back-to-every-line", check.Failed("squawk", 1, []byte("db/migrations/1.sql: warning\n"), []byte("found 1 issue\n")), []string{"db/migrations/1.sql: warning", "found 1 issue"}},
		{"red-failed-names-a-silent-exit", check.Failed("squawk", 3, nil, nil), []string{"squawk exited 3"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if !slices.Equal(tc.got, tc.want) {
				t.Fatalf("got %q, want %q", tc.got, tc.want)
			}
		})
	}
}
