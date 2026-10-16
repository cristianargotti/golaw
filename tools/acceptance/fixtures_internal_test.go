package main

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestClaudeMatrixRows(t *testing.T) {
	t.Parallel()
	names := make(map[string]bool)
	for _, item := range fixtures() {
		if names[item.name] {
			t.Fatalf("duplicate fixture: %s", item.name)
		}
		names[item.name] = true
	}
	for _, name := range []string{
		"dash-write", "clean-write", "dash-edit", "clean-edit",
		"secret-read", "source-read", "write-300", "write-301",
		"edit-300", "edit-301", "function-40", "function-41",
		"test-function-41", "malformed", "empty", "truncated", "oversized",
		"foreign-envelope", "notebook", "unreadable", "deleted-policy",
		"staged-weakening", "staged-deletion", "committed-policy", "unenrolled",
		"post-Write", "post-Edit", "post-NotebookEdit", "post-Bash", "failure-Bash",
		"post-PowerShell", "failure-PowerShell",
	} {
		if !names[name] {
			t.Errorf("missing fixture: %s", name)
		}
	}
	for _, count := range []int{1, 300, 301} {
		body := goLines(count)
		if strings.Count(body, "\n") != count || !strings.HasSuffix(body, "\n") {
			t.Fatalf("incorrect file line count %d", count)
		}
	}
	for _, count := range []int{40, 41} {
		set := token.NewFileSet()
		file, err := parser.ParseFile(set, "function.go", functionLines(count), 0)
		if err != nil {
			t.Fatal(err)
		}
		start := set.Position(file.Decls[0].Pos()).Line
		end := set.Position(file.Decls[0].End()).Line
		if end-start-1 != count {
			t.Fatalf("counted %d body lines, want %d", end-start-1, count)
		}
	}
}
