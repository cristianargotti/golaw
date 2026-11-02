package genimports_test

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/imports"
	"github.com/cristianargotti/golaw/internal/proof"
)

const kitModule = "github.com/cristianargotti/golaw"

// TestApplyLeavesTheKitsOwnSourceAsItIs runs the import fix over every Go
// file of the kit, a tree that builds and lints clean: any import it adds or
// removes there is a false fix, the defect zero friction forbids.
func TestApplyLeavesTheKitsOwnSourceAsItIs(t *testing.T) {
	t.Parallel()
	tree := kitTree(t, filepath.Join("..", ".."))
	x := imports.Std().With(kitModule, tree)
	checked := 0
	for _, f := range tree.Files() {
		content, err := tree.Read(f.Path)
		if err != nil {
			t.Fatal(err)
		}
		fix, err := imports.Apply(f.Path, content, x)
		if err != nil {
			t.Errorf("%s: %v", f.Path, err)
			continue
		}
		checked++
		if len(fix.Added) > 0 || len(fix.Removed) > 0 || !bytes.Equal(fix.Content, content) {
			t.Errorf("%s: a false fix, added %v removed %v", f.Path, fix.Added, fix.Removed)
		}
	}
	if checked < 100 {
		t.Fatalf("checked %d Go files, the kit holds hundreds: the walk lost its root", checked)
	}
}

// kitTree reads the Go files of the module, leaving out the fixtures and
// nested modules, which are no part of the kit's build.
func kitTree(t *testing.T, root string) *proof.Tree {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := filepath.ToSlash(strings.TrimPrefix(strings.TrimPrefix(p, root), string(filepath.Separator)))
		if d.IsDir() {
			return skipDir(root, p, rel)
		}
		if path.Ext(rel) != ".go" {
			return nil
		}
		content, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("read %s: %w", rel, err)
		}
		files[rel] = content
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return proof.NewTree(files)
}

func skipDir(root, p, rel string) error {
	if rel == "" {
		return nil
	}
	if slices.Contains([]string{"testdata", "vendor", "node_modules"}, path.Base(rel)) || strings.HasPrefix(path.Base(rel), ".") {
		return filepath.SkipDir
	}
	if _, err := os.Stat(filepath.Join(p, "go.mod")); err == nil && p != root {
		return filepath.SkipDir
	}
	return nil
}
