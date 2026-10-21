package policyfiles

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func makeInput(files map[string]string) model.Input {
	data := make(map[string][]byte, len(files))
	for name, body := range files {
		data[name] = []byte(body)
	}
	policy := model.CoreDefaults()
	return model.Input{
		Mode: model.ModeScan, Policy: &policy,
		Snapshot: proof.NewTree(data),
	}
}

func TestMakeReaderResolvesAndMerges(t *testing.T) {
	t.Parallel()
	const root = "Makefile"
	cases := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{
			"near modular",
			map[string]string{
				root:           "include mk/base.mk\nci: scan | cache\n",
				"mk/base.mk":   "include mk/checks.mk\nci: lint\n",
				"mk/checks.mk": "ci: test\n\tgo test ./...\n",
			},
			[]string{root, "mk/base.mk", "mk/checks.mk"},
		},
		{
			"near literal include glob",
			map[string]string{
				root:                   "include mk/*.mk\nci: scan\n",
				"mk/a.mk":              "ci: test\n\tgo test ./...\n",
				"mk/b.mk":              "ci: lint\n",
				"mk/nested/ignored.mk": "ci:: ignored\n",
			},
			[]string{root, "mk/a.mk", "mk/b.mk"},
		},
		{
			"near root relative nested include",
			map[string]string{
				root:         "include mk/base.mk\nci: scan\n",
				"mk/base.mk": "include checks.mk\nci: lint\n",
				"checks.mk":  "ci: test\n\tgo test ./...\n",
			},
			[]string{root, "mk/base.mk", "checks.mk"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := readMakefile(makeInput(tc.files), root)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got.files, tc.want) {
				t.Fatalf("files = %v, want %v", got.files, tc.want)
			}
			requireMergedMakeRule(t, got)
		})
	}
}

func requireMergedMakeRule(t *testing.T, got makefile) {
	t.Helper()
	rule, ok := got.rule("ci")
	if !ok || !slices.Equal(rule.prerequisites, []string{"test", "lint", "scan"}) {
		t.Fatalf("merged rule = %+v, found %v", rule, ok)
	}
	if !slices.Equal(rule.recipe, []string{"go test ./..."}) {
		t.Fatalf("recipe = %q", rule.recipe)
	}
	if _, ok := got.rule("cache"); ok {
		t.Fatal("an order-only prerequisite became a rule")
	}
}

func TestMakeReaderKeepsLocationsAndMissingIncludes(t *testing.T) {
	t.Parallel()
	const root = "Makefile"
	body := "# header\r\ninclude absent.mk\r\n-include optional.mk\r\n" +
		"sinclude optional-two.mk\r\nlint vet: input \\\r\n more | cache\r\n" +
		"\tgo vet ./...\r\nlint:\r\n\tgo vet -all ./...\r\n"
	got, err := readMakefile(makeInput(map[string]string{root: body}), root)
	if err != nil {
		t.Fatal(err)
	}
	want := []makeInclude{
		{name: "absent.mk", file: root, line: 2},
		{name: "optional.mk", optional: true, file: root, line: 3},
		{name: "optional-two.mk", optional: true, file: root, line: 4},
	}
	if !slices.Equal(got.missing, want) {
		t.Fatalf("missing = %+v", got.missing)
	}
	rule, ok := got.rule("lint")
	if !ok || rule.file != root || rule.line != 5 {
		t.Fatalf("location = %+v, found %v", rule, ok)
	}
	if !slices.Equal(rule.prerequisites, []string{"input", "more"}) {
		t.Fatalf("prerequisites = %q", rule.prerequisites)
	}
	if !slices.Equal(rule.recipe, []string{"go vet -all ./..."}) {
		t.Fatalf("winning recipe = %q", rule.recipe)
	}
}

func TestMakeReaderExportsAndOpaqueRecipes(t *testing.T) {
	t.Parallel()
	const root = "Makefile"
	for _, tc := range []struct {
		name, body, value string
		exported          bool
	}{
		{"near direct", "export GOTOOLCHAIN=local\n", "local", true},
		{"near separate", "GOTOOLCHAIN := local\nexport GOTOOLCHAIN\n", "local", true},
		{"near last assignment", "export GOTOOLCHAIN=auto\nGOTOOLCHAIN=local\n", "local", true},
		{"near not exported", "GOTOOLCHAIN=local\n", "local", false},
		{"near unexported", "export GOTOOLCHAIN=local\nunexport GOTOOLCHAIN\n", "local", false},
		{"near exported computed value", "export GOTOOLCHAIN=$(MODE)\n", "$(MODE)", true},
		{"near exported shell", "export GOTOOLCHAIN=$(shell go env GOVERSION)\n", "$(shell go env GOVERSION)", true},
		{"near unrelated computed export", "export PATH := $(CURDIR)/bin:$(PATH)\nexport GOTOOLCHAIN=local\n", "local", true},
		{"near known computed export", "MODE=local\nexport GOTOOLCHAIN := $(MODE)\n", "$(MODE)", true},
		{"near computed appended export", "MODE=local\nexport GOTOOLCHAIN:=prefix\nGOTOOLCHAIN+=$(MODE)\n", "prefix $(MODE)", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			files := map[string]string{
				root: tc.body + "UNUSED = $(shell touch marker)\n" +
					"ci: lint\nlint:\n\ttool $(if $(VALUE),yes,no)\n",
			}
			got, err := readMakefile(makeInput(files), root)
			if err != nil {
				t.Fatal(err)
			}
			value, exported := got.exported("GOTOOLCHAIN")
			if value != tc.value || exported != tc.exported {
				t.Fatalf("export = %q, %v", value, exported)
			}
			rule, ok := got.rule("lint")
			if !ok || !slices.Equal(rule.recipe, []string{"tool $(if $(VALUE),yes,no)"}) {
				t.Fatalf("opaque recipe = %+v", rule)
			}
		})
	}
}

func TestMakeReaderDoesNotInventAnAbsentRoot(t *testing.T) {
	t.Parallel()
	got, err := readMakefile(makeInput(nil), "Makefile")
	if err != nil || len(got.files) != 0 || len(got.rules) != 0 {
		t.Fatalf("absent root = %+v, %v", got, err)
	}
}

func TestMakeReaderKeepsRecipeContinuations(t *testing.T) {
	t.Parallel()
	for _, prefix := range []string{"    ", "\t", "\t  ", ""} {
		t.Run("indent="+prefix, func(t *testing.T) {
			t.Parallel()
			body := "ci: lint\n\tdocker run \\\n" + prefix + "-v $(PWD):/src image # kept \\\n\t# continuation\n\techo done # recipe\n"
			got, err := readMakefile(makeInput(map[string]string{"Makefile": body}), "Makefile")
			if err != nil || len(got.rules) != 1 {
				t.Fatalf("continued recipe rules = %+v, %v", got.rules, err)
			}
			want := []string{"docker run \\\n" + strings.TrimPrefix(prefix, "\t") + "-v $(PWD):/src image # kept \\\n# continuation", "echo done # recipe"}
			rule, ok := got.rule("ci")
			if !ok || rule.line != 1 || rule.file != "Makefile" || !slices.Equal(rule.recipe, want) {
				t.Fatalf("continued recipe = %+v, found %v", rule, ok)
			}
		})
	}
}
