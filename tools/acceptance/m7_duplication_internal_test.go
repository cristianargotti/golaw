package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/dup"
	"github.com/cristianargotti/golaw/internal/newcode"
)

// m7MeasuredGo reads every Go file of root the pipeline analyses: third
// party output under a gen or sqlcgen directory is excluded by its globs.
func m7MeasuredGo(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("relate %s: %w", path, err)
		}
		slash := filepath.ToSlash(rel)
		if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == "gen" || entry.Name() == "sqlcgen") {
			return filepath.SkipDir
		}
		if entry.Type().IsRegular() && strings.HasSuffix(slash, ".go") {
			body, readErr := readFile(path)
			files[slash] = body
			return readErr
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestM7Duplication(t *testing.T) {
	t.Parallel()
	root, base := os.Getenv(m7DupRepoKey), os.Getenv(m7DupBaseKey)
	if root == "" || base == "" {
		t.Skip("the m7-fresh/duplication row sets the repository and its base commit")
	}
	lines, err := newcode.NewLines(t.Context(), root, base)
	if err != nil {
		t.Fatal(err)
	}
	files := m7MeasuredGo(t, root)
	for path := range lines {
		if _, measured := files[path]; !measured {
			delete(lines, path)
		}
	}
	report, err := dup.Measure(files, lines, dup.Default())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s %.2f", m7DupMarker, report.Percent())
	if report.Percent() >= m7MaxDuplication {
		for _, block := range report.Blocks {
			t.Errorf("duplicated block of %d tokens at %v", block.Tokens, slices.Clone(block.Spans))
		}
	}
}
