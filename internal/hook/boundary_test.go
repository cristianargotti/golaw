package hook_test

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func prohibitedExecutionImport(t *testing.T, source []byte) bool {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "input.go", source, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, imported := range file.Imports {
		path, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		switch path {
		case "os/exec", "syscall", "unsafe", "plugin",
			"github.com/cristianargotti/golaw/internal/proof":
			return true
		}
	}
	return false
}

func TestHookExecutionBoundary(t *testing.T) {
	t.Parallel()
	if !prohibitedExecutionImport(t, []byte("package p\nimport \"os/exec\"\n")) {
		t.Fatal("red boundary fixture passed")
	}
	if prohibitedExecutionImport(t, []byte("package p\nimport \"encoding/json\"\n")) {
		t.Fatal("near-miss boundary fixture failed")
	}
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") ||
			strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if prohibitedExecutionImport(t, data) {
			t.Errorf("hook execution boundary crossed: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
