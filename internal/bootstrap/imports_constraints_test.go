package bootstrap_test

import (
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
)

func TestPromoteImportedEvaluatesConstraintExpressions(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, header string
		want         bool
	}{
		{"modern wins inclusion", "//go:build tools\n// +build ignore\n\n", true},
		{"modern wins exclusion", "//go:build ignore\n// +build tools\n\n", false},
		{"legacy lines intersect", "// +build tools\n// +build ignore\n\n", false},
		{"legacy alternatives", "// +build ignore tools\n\n", true},
		{"legacy conjunction", "// +build ignore,tools\n\n", false},
		{"negated ignore", "//go:build !ignore\n\n", true},
		{"nested negation", "//go:build !(ignore || linux)\n\n", true},
		{"conservative conjunction", "//go:build linux && !linux\n\n", true},
		{"excluded conjunction", "//go:build ignore && tools\n\n", false},
		{"excluded alternatives", "//go:build ignore || ignore\n\n", false},
		{"malformed modern constraint", "//go:build (\n\n", true},
		{"multiple modern constraints", "//go:build ignore\n//go:build tools\n\n", true},
		{"block comment", "/*\n//go:build ignore\n*/\n\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeBefore(t, filepath.Join(root, "gen.go"), tc.header+"package main\n\nimport _ \"example.test/cli\"\n")
			info, promoted, err := bootstrap.PromoteImported(root, importsModule())
			if err != nil {
				t.Fatal(err)
			}
			var want []string
			if tc.want {
				want = []string{"example.test/cli"}
			}
			requireImported(t, info, promoted, want)
		})
	}
}

func TestPromoteImportedKeepsCommentsAfterThePackageOutOfConstraints(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeBefore(t, filepath.Join(root, "gen.go"), "package main\n\n//go:build ignore\n\nimport _ \"example.test/cli\"\n")
	info, promoted, err := bootstrap.PromoteImported(root, importsModule())
	if err != nil {
		t.Fatal(err)
	}
	requireImported(t, info, promoted, []string{"example.test/cli"})
}
