package bootstrap_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/install"
)

func TestIrregularPathsNameLinkedAndMistypedDestinations(t *testing.T) {
	t.Parallel()
	root, outside := t.TempDir(), t.TempDir()
	writeBefore(t, filepath.Join(root, "tools"), "regular\n")
	writeBefore(t, filepath.Join(root, "Makefile"), "all:\n")
	writeBefore(t, filepath.Join(outside, "editorconfig"), "root = true\n")
	if err := os.Mkdir(filepath.Join(root, "lefthook.yml"), 0o700); err != nil {
		t.Fatal(err)
	}
	for link, target := range map[string]string{".editorconfig": filepath.Join(outside, "editorconfig"), "docs": outside} {
		if err := os.Symlink(target, filepath.Join(root, link)); err != nil {
			t.Fatal(err)
		}
	}
	paths := []string{
		"golaw.mk", "Makefile", ".editorconfig", "docs/adr/0001-inherited-baseline.md",
		"tools/go.mod", "lefthook.yml", "nested/new.txt",
	}
	got, err := bootstrap.IrregularPaths(root, paths)
	want := []string{".editorconfig", "docs/adr/0001-inherited-baseline.md", "tools/go.mod", "lefthook.yml"}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("irregular paths = %v, %v", got, err)
	}
	if _, err := bootstrap.IrregularPaths(root, []string{"../escape.txt"}); !errors.Is(err, install.ErrPath) {
		t.Fatalf("invalid destination = %v", err)
	}
	if _, err := bootstrap.IrregularPaths(filepath.Join(root, "missing"), paths); err == nil {
		t.Fatal("a missing root was accepted")
	}
}

func TestTemplateDiffReportsIrregularDestinations(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ language, protected, irregular string }{
		{
			"pt-BR", "Destino protegido por CODEOWNERS, preservado: golaw.yml.\n",
			"Destino com link simbólico ou caminho irregular, preservado: tools/go.mod.\n",
		},
		{
			"en", "Destination protected by CODEOWNERS, preserved: golaw.yml.\n",
			"Destination with a symbolic link or an irregular path, preserved: tools/go.mod.\n",
		},
	} {
		t.Run(tc.language, func(t *testing.T) {
			t.Parallel()
			plan := bootstrap.TemplatePlan{
				Language: tc.language, Irregular: []string{"tools/go.mod"},
				Ownership: bootstrap.Ownership{Protected: []string{"golaw.yml"}},
			}
			var out bytes.Buffer
			if err := bootstrap.WriteTemplateDiff(&out, plan); err != nil {
				t.Fatal(err)
			}
			if out.String() != tc.protected+tc.irregular {
				t.Fatalf("irregular report = %q", out.String())
			}
		})
	}
}
