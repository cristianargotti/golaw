package policyfiles

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestMakeReaderNamesItsLimits(t *testing.T) {
	t.Parallel()
	const root = "Makefile"
	for _, tc := range []struct {
		name, body, construct string
	}{
		{"red computed include", "include $(PART)\n", "$(PART)"},
		{"red cycle", "include Makefile\n", "cycle"},
		{"red double colon", "ci:: lint\n", "::"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := readMakefile(makeInput(map[string]string{root: tc.body}), root)
			if !errors.Is(err, model.ErrOutsideSubset) {
				t.Fatalf("limit = %v", err)
			}
			if !strings.Contains(err.Error(), root+":") ||
				!strings.Contains(err.Error(), tc.construct) {
				t.Fatalf("limit does not name its source: %v", err)
			}
		})
	}
}

// TestMakeReaderRecordsConditionalDefinitions pins the reading of a
// conditional block: its rules, includes and exports are recorded with the
// location of their opener, never merged into the unconditional reading and
// never a limit of the reader, so the dotenv include and the per-OS export
// of a common Go Makefile leave every build check measured.
func TestMakeReaderRecordsConditionalDefinitions(t *testing.T) {
	t.Parallel()
	const root = "Makefile"
	got, err := readMakefile(makeInput(map[string]string{
		root: "ifeq ($(MODE),fast)\nci: lint\nexport EXE=.exe\nendif\n" +
			"ifneq ($(wildcard local.mk),)\ninclude local.mk\nendif\n" +
			"ifdef DEPLOY\ninclude deploy.mk\nendif\n" +
			"export GOTOOLCHAIN=local\nlint:\n",
		"deploy.mk": "deploy: build\n",
	}), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.rule("ci"); ok {
		t.Fatal("a rule inside a conditional block merged into the unconditional rules")
	}
	for target, opener := range map[string]string{"ci": "Makefile:1", "deploy": "Makefile:8"} {
		if rule, ok := got.conditional(target); !ok || rule.condition != opener {
			t.Fatalf("conditional %s = %+v, found %v", target, rule, ok)
		}
	}
	if rule, ok := got.rule("lint"); !ok || rule.condition != "" {
		t.Fatalf("unconditional rule = %+v, found %v", rule, ok)
	}
	want := []makeInclude{{name: "local.mk", file: root, line: 6, condition: "Makefile:5"}}
	if !slices.Equal(got.missing, want) {
		t.Fatalf("missing = %+v", got.missing)
	}
	if value, exported := got.exported("GOTOOLCHAIN"); value != "local" || !exported {
		t.Fatalf("unconditional export = %q, %v", value, exported)
	}
	if _, exported := got.exported("EXE"); exported {
		t.Fatal("a conditional export answered as unconditional")
	}
	if v, ok := got.conditionalVar("EXE"); !ok || v.condition != "Makefile:1" || !v.exported {
		t.Fatalf("conditional export = %+v, found %v", v, ok)
	}
}

// TestMakeReaderKeepsFunctionsDefinesAndTargetVariablesApart pins the three
// forms make reads that are neither rules nor global assignments: a
// top-level function line is recorded, never evaluated; a define block is
// one assignment whose body holds no rule; a target-specific assignment is a
// variable of its target, never a prerequisite and never global.
func TestMakeReaderKeepsFunctionsDefinesAndTargetVariablesApart(t *testing.T) {
	t.Parallel()
	const root = "Makefile"
	got, err := readMakefile(makeInput(map[string]string{
		root: "" +
			"define RULE\n$(1):\n\tgo test ./...\nendef\n" +
			"$(eval $(call RULE,unit))\n" +
			"lint: export GOTOOLCHAIN := auto\n" +
			"test: VERBOSE = -v\n" +
			"ci: lint test\nlint:\n\tgo run tool@v1.0.0\ntest:\n\tgo test $(VERBOSE) ./...\n",
	}), root)
	if err != nil {
		t.Fatal(err)
	}
	if want := []makeOpaque{{text: "$(eval $(call RULE,unit))", file: root, line: 5}}; !slices.Equal(got.opaque, want) {
		t.Fatalf("opaque = %+v", got.opaque)
	}
	if _, ok := got.rule("$(1)"); ok {
		t.Fatal("a line of a define block read as a rule")
	}
	for target, recipe := range map[string][]string{"lint": {"go run tool@v1.0.0"}, "test": {"go test $(VERBOSE) ./..."}} {
		rule, ok := got.rule(target)
		if !ok || len(rule.prerequisites) != 0 || !slices.Equal(rule.recipe, recipe) {
			t.Fatalf("rule %s = %+v, found %v", target, rule, ok)
		}
	}
	if _, exported := got.exported("GOTOOLCHAIN"); exported {
		t.Fatal("a target-specific export answered as global")
	}
	var scoped []string
	for _, v := range got.vars {
		if v.target != "" {
			scoped = append(scoped, v.target+":"+v.name+"="+v.value)
		}
	}
	if !slices.Equal(scoped, []string{"lint:GOTOOLCHAIN=auto", "test:VERBOSE=-v"}) {
		t.Fatalf("target-specific variables = %v", scoped)
	}
}

func TestMakeReaderSkipsIrrelevantExpressions(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		"UNUSED = $(shell touch marker)\nci: lint\nlint:\n",
		"# include $(UNKNOWN)\nci: lint\nlint:\n",
		"ci: lint\nlint:\n\tprintf '%s' '$(shell ignored-by-this-reader)'\n",
	} {
		got, err := readMakefile(makeInput(map[string]string{"Makefile": body}), "Makefile")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := got.rule("ci"); !ok {
			t.Fatal("the literal rule disappeared")
		}
	}
}

func TestMakeReaderPropagatesTheIncludeReadBound(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"Makefile": "include big.mk\n",
		"big.mk":   string(make([]byte, model.MaxReadBytes+1)),
	}
	_, err := readMakefile(makeInput(files), "Makefile")
	if !errors.Is(err, model.ErrTooLarge) || !strings.Contains(err.Error(), "big.mk") {
		t.Fatalf("include read error = %v", err)
	}
}

func TestMakeReaderIncludeAliasesAndMalformedGlobs(t *testing.T) {
	t.Parallel()
	got, err := readMakefile(makeInput(map[string]string{
		"Build.mk":     "include mk/./checks.mk\ninclude mk/checks.mk\n",
		"mk/checks.mk": "ci: lint\n",
	}), "Build.mk")
	if err != nil || !slices.Equal(got.files, []string{"Build.mk", "mk/checks.mk", "mk/checks.mk"}) {
		t.Fatalf("repeated includes = %+v, %v", got, err)
	}
	rule, ok := got.rule("ci")
	if !ok || !slices.Equal(rule.prerequisites, []string{"lint", "lint"}) {
		t.Fatalf("repeated rule = %+v, %v", rule, ok)
	}
	_, err = readMakefile(makeInput(map[string]string{"Makefile": "include ./Makefile\n"}), "Makefile")
	if !errors.Is(err, model.ErrOutsideSubset) || !strings.Contains(err.Error(), "Makefile:1") {
		t.Fatalf("normalized cycle = %v", err)
	}
	_, err = readMakefile(makeInput(map[string]string{"Makefile": "include missing/[\n"}), "Makefile")
	if err == nil || errors.Is(err, model.ErrOutsideSubset) || !strings.Contains(err.Error(), "missing/[") {
		t.Fatalf("malformed glob = %v", err)
	}
}

func TestMakeReaderKeepsPOSIXTargetAssignmentsScoped(t *testing.T) {
	t.Parallel()
	for _, op := range []string{"::=", ":::="} {
		t.Run(op, func(t *testing.T) {
			t.Parallel()
			got, err := readMakefile(makeInput(map[string]string{"Makefile": "lint: export X " + op + " v\nlint:\n"}), "Makefile")
			if err != nil || !slices.Equal(got.vars, []makeVar{{name: "X", value: "v", exported: true, target: "lint", file: "Makefile", line: 1}}) {
				t.Fatalf("target-specific assignment = %+v, %v", got.vars, err)
			}
			rule, ok := got.rule("lint")
			if !ok || len(got.rules) != 1 || len(rule.prerequisites) != 0 {
				t.Fatalf("target-specific rule = %+v, found %v", rule, ok)
			}
			if _, exported := got.exported("X"); exported {
				t.Fatal("a target-specific POSIX assignment answered as global")
			}
		})
	}
}
