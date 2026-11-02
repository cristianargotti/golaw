package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestM7FillsRefuseAnIncompleteRecord: no recorded file, a fill without a
// body and a document that does not decode are each an error.
func TestM7FillsRefuseAnIncompleteRecord(t *testing.T) {
	t.Parallel()
	if _, err := m7Fills(t.TempDir()); err == nil {
		t.Fatal("an absent record decoded")
	}
	for name, body := range map[string]string{
		"red a fill without a body": "- id: a.b\n  body: \"\"\n",
		"red a fill without an id":  "- body: x\n",
		"red not a list":            "id: a.b\n",
	} {
		dir := t.TempDir()
		m2Write(t, filepath.Join(dir, filepath.FromSlash(m7FixtureRoot), m7FillsFile+".esc"), body)
		if _, err := m7Fills(dir); err == nil {
			t.Fatalf("%s: decoded", name)
		}
	}
}

// TestM7PruneImportsDropsWhatOnlyTheHoleUsed: an import the filled body
// no longer names is dropped, in a block or alone, and a blank, a dot and
// a used import stay.
func TestM7PruneImportsDropsWhatOnlyTheHoleUsed(t *testing.T) {
	t.Parallel()
	src := "package a\n\nimport (\n\t_ \"embed\"\n\t. \"strings\"\n\t\"fmt\"\n\tsup \"example.com/x/gensupport\"\n\t\"errors\"\n)\n\nvar V = errors.New(ToUpper(\"v\"))\n"
	got, err := m7PruneImports([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	if strings.Contains(text, "\"fmt\"") || strings.Contains(text, "gensupport") || !strings.Contains(text, "_ \"embed\"") ||
		!strings.Contains(text, ". \"strings\"") || !strings.Contains(text, "\"errors\"") {
		t.Fatalf("pruned =\n%s", text)
	}
	got, err = m7PruneImports([]byte("package a\n\nimport \"fmt\"\n\nvar V = 1\n"))
	if err != nil || strings.Contains(string(got), "import") {
		t.Fatalf("a lone unused import = %v\n%s", err, got)
	}
	if _, err := m7PruneImports([]byte("package a\n\nfunc (\n")); err == nil {
		t.Fatal("a file that does not parse was pruned")
	}
}

// TestM7FillRefusesWhatDoesNotParse: a file or an import list that does not
// parse is named, and an import a fill names twice is added once.
func TestM7FillRefusesWhatDoesNotParse(t *testing.T) {
	t.Parallel()
	if _, err := m7FillFunc([]byte("package (\n"), m7Fill{ID: "a.b"}); err == nil || !strings.Contains(err.Error(), "a.b") {
		t.Fatalf("a file that does not parse = %v", err)
	}
	if _, err := m7AddImports([]byte("package (\n"), []m7Import{{Path: "errors"}}); err == nil {
		t.Fatal("imports were added to a file that does not parse")
	}
	got, err := m7AddImports([]byte("package a\n"), []m7Import{{Path: "errors"}, {Path: "errors"}})
	if err != nil || strings.Count(string(got), "\"errors\"") != 1 {
		t.Fatalf("a twice named import = %v\n%s", err, got)
	}
	if _, err := m7AddImports([]byte("package a\n\nfunc (\n"), []m7Import{{Path: "errors"}}); err == nil {
		t.Fatal("a body that does not parse was formatted")
	}
}

// TestM7ApplyFillsStopsAtTheFirstFailure: an unreadable tree, a fill that
// does not place and a file that cannot be written are each an error.
func TestM7ApplyFillsStopsAtTheFirstFailure(t *testing.T) {
	t.Parallel()
	if err := m7ApplyFills(filepath.Join(t.TempDir(), "absent"), nil); err == nil {
		t.Fatal("an absent tree was filled")
	}
	dir := t.TempDir()
	m2Write(t, filepath.Join(dir, "plans.go"), m7Owned)
	if err := m7ApplyFills(dir, []m7Fill{{ID: "plans.read-plan", Recv: "p"}}); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("a fill of the wrong arity = %v", err)
	}
	fill := m7Fill{ID: "plans.read-plan", Recv: "p", Params: []string{"ctx", "account"}, Body: "return account, ctx.Err()\n"}
	m2Write(t, filepath.Join(dir, ".git", "hooks", "skip.go"), m7Owned)
	if err := m7ApplyFills(dir, []m7Fill{fill}); err != nil {
		t.Fatalf("a hole the git directory repeats = %v", err)
	}
	m2Write(t, filepath.Join(dir, "plans.go"), m7Owned)
	if err := os.Chmod(filepath.Join(dir, "plans.go"), 0o400); err != nil {
		t.Fatal(err)
	}
	if err := m7ApplyFills(dir, []m7Fill{fill}); err == nil && os.Getuid() != 0 {
		t.Fatal("a read only file was written")
	}
}
