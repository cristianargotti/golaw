package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The route of a removal: the agent deletes the owned files, the pipeline
// names the removal golaw gen has not run yet, and golaw gen removes the
// entries and their regenerable files.
func TestGenRemovesTheEntriesWhoseOwnedFilesTheAgentDeleted(t *testing.T) {
	t.Parallel()
	ta, root := newPort(t)
	for _, owned := range []string{
		"internal/application/growth/ledgertest/cases.go", "internal/application/growth/read_plan.go",
		"internal/application/growth/read_plan_test.go",
	} {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(owned))); err != nil {
			t.Fatal(err)
		}
	}
	fake := "internal/application/growth/ledgertest/zz_generated_fake.go"
	if code := ta.run(t.Context(), []string{"gen", "--check"}); code != exitFindings ||
		!strings.Contains(ta.stdout.String(), "drift "+fake+" belongs to an entry whose owned files are gone: run golaw gen") {
		t.Fatalf("check exit = %d: %s", code, ta.stdout.String())
	}
	ta.stdout.Reset()
	if code := ta.run(t.Context(), []string{"gen"}); code != exitClean {
		t.Fatalf("gen exit = %d: %s%s", code, ta.stdout.String(), ta.stderr.String())
	}
	for _, want := range []string{"removed port growth.Ledger", "removed usecase growth.ReadPlan"} {
		if !strings.Contains(ta.stdout.String(), want) {
			t.Errorf("stdout lacks %q: %s", want, ta.stdout.String())
		}
	}
	if !strings.Contains(ta.stderr.String(), "entrada removida port growth.Ledger") {
		t.Errorf("the person's line is missing: %q", ta.stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(fake))); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the regenerable file of the removed port remains: %v", err)
	}
	ta.stdout.Reset()
	ta.run(t.Context(), []string{"gen", "--check"})
	if strings.Contains(ta.stdout.String(), "drift ") {
		t.Errorf("drift after the removal:\n%s", ta.stdout.String())
	}
}
