package imports_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/imports"
	"github.com/cristianargotti/golaw/internal/proof"
)

const module = "example.com/slice"

// moduleIndex is the standard library with the packages of a small module:
// growth, and a package whose clause differs from its directory.
func moduleIndex() imports.Index {
	tree := proof.NewTree(map[string][]byte{
		"go.mod":                                   []byte("module example.com/slice\n\ngo 1.27\n"),
		"internal/application/growth/plan.go":      []byte("package growth\n\ntype Plan struct{}\n"),
		"internal/version/release.go":              []byte("package release\n\nconst Name = \"x\"\n"),
		"cmd/slice/main.go":                        []byte("package main\n\nfunc main() {}\n"),
		"internal/gen/testdata/fixture/fixture.go": []byte("package fixture\n"),
	})
	return imports.Std().With(module, tree)
}

func TestApplyFixesOnlyTheCertainImports(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		in, want       string
		added, removed []string
		report         []string
	}{
		"red: removes an import no selector names": {
			in:      "package x\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\nfunc f() { fmt.Println() }\n",
			want:    "package x\n\nimport (\n\t\"fmt\"\n)\n\nfunc f() { fmt.Println() }\n",
			removed: []string{"strings"},
		},
		"red: removes a single import decl of a major version path": {
			in:      "package x\n\nimport \"math/rand/v2\"\n\nfunc f() {}\n",
			want:    "package x\n\nfunc f() {}\n",
			removed: []string{"math/rand/v2"},
		},
		"red: adds a standard package into the block": {
			in:    "package x\n\nimport (\n\t\"fmt\"\n)\n\nfunc f() { fmt.Println(strings.ToUpper(\"a\")) }\n",
			want:  "package x\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\nfunc f() { fmt.Println(strings.ToUpper(\"a\")) }\n",
			added: []string{"strings"},
		},
		"red: adds a block to a file without imports": {
			in:    "package x\n\nfunc f() string { return strings.ToUpper(\"a\") }\n",
			want:  "package x\n\nimport (\n\t\"strings\"\n)\n\nfunc f() string { return strings.ToUpper(\"a\") }\n",
			added: []string{"strings"},
		},
		"red: adds a package of the module as its own group": {
			in:    "package postgres\n\nimport (\n\t\"context\"\n)\n\nfunc read(ctx context.Context) growth.Plan { return growth.Plan{} }\n",
			want:  "package postgres\n\nimport (\n\t\"context\"\n\n\t\"example.com/slice/internal/application/growth\"\n)\n\nfunc read(ctx context.Context) growth.Plan { return growth.Plan{} }\n",
			added: []string{"example.com/slice/internal/application/growth"},
		},
		"near: an ambiguous name is reported, never guessed": {
			in:     "package x\n\nfunc f() int { return rand.Int() }\n",
			want:   "package x\n\nfunc f() int { return rand.Int() }\n",
			report: []string{"rand: crypto/rand or math/rand or math/rand/v2"},
		},
		"near: a local identifier shadows the package": {
			in:   "package x\n\nfunc f() int {\n\tstrings := counter{}\n\treturn strings.Len()\n}\n",
			want: "package x\n\nfunc f() int {\n\tstrings := counter{}\n\treturn strings.Len()\n}\n",
		},
		"near: a path whose package name is unknown stays": {
			in:     "package x\n\nimport \"gopkg.in/yaml.v3\"\n\nfunc f() {}\n",
			want:   "package x\n\nimport \"gopkg.in/yaml.v3\"\n\nfunc f() {}\n",
			report: []string{"gopkg.in/yaml.v3: package name unknown, left as it is"},
		},
		"near: a package named apart from its directory stays": {
			in:     "package x\n\nimport \"example.com/slice/internal/version\"\n\nfunc f() {}\n",
			want:   "package x\n\nimport \"example.com/slice/internal/version\"\n\nfunc f() {}\n",
			report: []string{"example.com/slice/internal/version: unused under release and version, left as it is"},
		},
		"near: a blank import is never touched": {
			in:   "package x\n\nimport _ \"embed\"\n",
			want: "package x\n\nimport _ \"embed\"\n",
		},
		"near: a named import in use stays": {
			in:   "package x\n\nimport str \"strings\"\n\nfunc f() string { return str.ToUpper(\"a\") }\n",
			want: "package x\n\nimport str \"strings\"\n\nfunc f() string { return str.ToUpper(\"a\") }\n",
		},
	}
	x := moduleIndex()
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fix, err := imports.Apply("x.go", []byte(tc.in), x)
			if err != nil {
				t.Fatal(err)
			}
			if string(fix.Content) != tc.want {
				t.Errorf("Content =\n%s\nwant\n%s", fix.Content, tc.want)
			}
			if !slices.Equal(fix.Added, tc.added) || !slices.Equal(fix.Removed, tc.removed) || !slices.Equal(fix.Report, tc.report) {
				t.Errorf("Added %v Removed %v Report %v, want %v %v %v", fix.Added, fix.Removed, fix.Report, tc.added, tc.removed, tc.report)
			}
		})
	}
}

func TestApplyNamesTheParserLineOfAFileThatDoesNotParse(t *testing.T) {
	t.Parallel()
	_, err := imports.Apply("x.go", []byte("package x\n\nfunc {\n"), imports.Std())
	if !errors.Is(err, imports.ErrParse) || !strings.Contains(err.Error(), "x.go:3:") {
		t.Fatalf("err = %v, want ErrParse naming x.go:3", err)
	}
}
