package policyfiles

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestMakeReaderExpandsOnlyLiteralAssignments(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body, target string
		want               []string
		line               int
	}{
		{"near variable prerequisite", "CHECKS=lint\nci: $(CHECKS)\n", "ci", []string{"lint"}, 2},
		{"near assignment flavors", "BASE=lint\nCHECKS ?= $(BASE)\nCHECKS += vet\nci: $(CHECKS)\n", "ci", []string{"lint", "vet"}, 4},
		{"near immediate value", "BASE=lint\nCHECKS := $(BASE)\nBASE=vet\nci: $(CHECKS)\n", "ci", []string{"lint"}, 4},
		{"near posix simple assignment", "CHECKS ::= lint\nci: $(CHECKS)\n", "ci", []string{"lint"}, 2},
		{"near posix immediate assignment", "CHECKS :::= lint\nci: $(CHECKS)\n", "ci", []string{"lint"}, 2},
		{"near shell assignment", "NOW != date\nci: lint $(NOW)\n", "ci", []string{"lint", "$(NOW)"}, 2},
		{"near overwritten shell assignment", "NOW != date\nNOW=lint\nci: $(NOW)\n", "ci", []string{"$(NOW)"}, 3},
		{"near posix simple freeze", "BASE=lint\nCHECKS ::= $(BASE)\nBASE=vet\nci: $(CHECKS)\n", "ci", []string{"lint"}, 4},
		{"near posix immediate freeze", "BASE=lint\nCHECKS :::= $(BASE)\nBASE=vet\nci: $(CHECKS)\n", "ci", []string{"lint"}, 4},
		{"near recursive value", "BASE=lint\nCHECKS=$(BASE)\nBASE=vet\nci: $(CHECKS)\n", "ci", []string{"vet"}, 4},
		{"near default retained", "CHECKS=lint\nCHECKS ?= vet\nci: $(CHECKS)\n", "ci", []string{"lint"}, 3},
		{"near pattern rule", "%: base\n", "%", []string{"base"}, 1},
		{"near raw target", "$(BIN): $(SRC)\n", "$(BIN)", []string{"$(SRC)"}, 1},
		{"near shell prerequisite", "ci: $(shell printf lint)\n", "ci", []string{"$(shell printf lint)"}, 1},
		{"near function prerequisite", "ci: $(sort lint vet)\n", "ci", []string{"$(sort lint vet)"}, 1},
		{"near unknown variable", "ci: $(CHECKS)\n", "ci", []string{"$(CHECKS)"}, 1},
		{"near variable cycle", "CHECKS=$(CHECKS)\nci: $(CHECKS)\n", "ci", []string{"$(CHECKS)"}, 2},
		{"near conditional assignment", "ifeq ($(OS),Windows_NT)\nEXE=.exe\nendif\nci: lint\n", "ci", []string{"lint"}, 4},
		{"near conditional reference", "ifeq ($(OS),Windows_NT)\nEXE=.exe\nendif\nci: $(EXE)\n", "ci", []string{"$(EXE)"}, 4},
		{"near tainted assignment", "CHECKS=$(shell printf lint)\nCHECKS=lint\nci: $(CHECKS)\n", "ci", []string{"$(CHECKS)"}, 3},
		{"near immediate append", "BASE=lint\nCHECKS:=$(BASE)\nCHECKS+=$(BASE)\nBASE=vet\nci: ${CHECKS}\n", "ci", []string{"lint", "lint"}, 5},
		{"near recursive append", "BASE=lint\nCHECKS=$(BASE)\nCHECKS+=$(BASE)\nBASE=vet\nci: pre-${CHECKS}\n", "ci", []string{"pre-vet", "vet"}, 5},
		{"near later tainted dependency", "BASE=lint\nCHECKS:=$(BASE)\nci: $(CHECKS)\nBASE=$(shell false)\n", "ci", []string{"$(CHECKS)"}, 3},
		{"near overwritten environment reference", "CHECKS=$(UNKNOWN)\nCHECKS=lint\nci: $(CHECKS)\n", "ci", []string{"$(CHECKS)"}, 3},
		{"near later literal dependency", "CHECKS=$(BASE)\nBASE=lint\nci: $(CHECKS)\n", "ci", []string{"lint"}, 3},
		{"near partial expansion", "BASE=lint\nci: $(BASE)${UNKNOWN}\n", "ci", []string{"$(BASE)${UNKNOWN}"}, 2},
		{"near unclosed parenthesis", "ci: $(\n", "ci", []string{"$("}, 1},
		{"near unclosed brace", "ci: ${\n", "ci", []string{"${"}, 1},
		{"near mismatched closer", "BASE=lint\nci: $(BASE}\n", "ci", []string{"$(BASE}"}, 2},
		{"near short variable", "ci: $X\n", "ci", []string{"$X"}, 1},
		{"near nested function", "ci: $(if ${A},$(sort lint vet),test)\n", "ci", []string{"$(if ${A},$(sort lint vet),test)"}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := readMakefile(makeInput(map[string]string{"Makefile": tc.body}), "Makefile")
			if err != nil || len(got.rules) != 1 {
				t.Fatalf("read = %+v, %v", got, err)
			}
			if tc.name == "near shell assignment" && !slices.Equal(got.vars, []makeVar{{name: "NOW", value: "date", file: "Makefile", line: 1}}) {
				t.Fatalf("shell assignment = %+v", got.vars)
			}
			rule := got.rules[0]
			if !slices.Equal(rule.targets, []string{tc.target}) ||
				!slices.Equal(rule.prerequisites, tc.want) ||
				rule.file != "Makefile" || rule.line != tc.line {
				t.Fatalf("rule = %+v", rule)
			}
		})
	}
}

func TestMakeReaderBoundsLiteralExpansion(t *testing.T) {
	t.Parallel()
	body := make([]string, 0, 34)
	for i := range 33 {
		body = append(body, fmt.Sprintf("V%d=$(V%d)\n", i, i+1))
	}
	body = append(body, "V33=lint\nci: $(V0)\n")
	got, err := readMakefile(makeInput(map[string]string{"Makefile": strings.Join(body, "")}), "Makefile")
	if err != nil {
		t.Fatal(err)
	}
	rule, ok := got.rule("ci")
	if !ok || !slices.Equal(rule.prerequisites, []string{"$(V0)"}) {
		t.Fatalf("bounded expansion = %+v", rule)
	}
}

func TestMakeReaderRecordsUnmatchedGlobs(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, directive, pattern string
		optional                 bool
	}{
		{"red required glob", "include", "mk/*.mk", false},
		{"near optional glob", "-include", "mk/?.mk", true},
		{"near optional bracket glob", "sinclude", "mk/[ab].mk", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := tc.directive + " " + tc.pattern + "\n"
			got, err := readMakefile(makeInput(map[string]string{"Makefile": body}), "Makefile")
			want := []makeInclude{{name: tc.pattern, optional: tc.optional, file: "Makefile", line: 1}}
			if err != nil || !slices.Equal(got.missing, want) {
				t.Fatalf("missing = %+v, %v", got.missing, err)
			}
		})
	}
}

func TestMakeReaderRejectsUnsafeIncludePaths(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"../outside.mk", "/opt/rules.mk", "C:/rules.mk"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			body := "include " + name + "\n"
			_, err := readMakefile(makeInput(map[string]string{"Makefile": body}), "Makefile")
			if !errors.Is(err, model.ErrUnreadable) || errors.Is(err, model.ErrOutsideSubset) {
				t.Fatalf("unsafe include = %v", err)
			}
			if !strings.Contains(err.Error(), name) {
				t.Fatalf("include path missing from error: %v", err)
			}
		})
	}
}

func TestMakeReaderConditionalBranchesAndOpaqueBodies(t *testing.T) {
	t.Parallel()
	got, err := readMakefile(makeInput(map[string]string{
		"Makefile": "ifdef OUTER\ninclude inner.mk\nelse ifndef OTHER\nother:\nendif\n" +
			"define OUTER\ndefine INNER\nfalse-rule:\nendef\nouter body\nendef\n" +
			"ci: lint\n\tinclude false.mk\n\tendif\n\tX=literal # recipe\n" +
			"ci:\n\t\n# continued comment \\\nfalse-rule:\nlint:\n",
		"inner.mk": "ifdef INNER\ninner:\nelse\nalternate:\nendif\ninherited:\n",
	}), "Makefile")
	if err != nil {
		t.Fatal(err)
	}
	for target, location := range map[string]string{
		"inner": "inner.mk:1", "alternate": "inner.mk:1", "inherited": "Makefile:1", "other": "Makefile:1",
	} {
		if rule, ok := got.conditional(target); !ok || rule.condition != location {
			t.Fatalf("conditional %s = %+v, %v", target, rule, ok)
		}
	}
	rule, ok := got.rule("ci")
	if !ok || !slices.Equal(rule.recipe, []string{""}) || len(got.missing) != 0 {
		t.Fatalf("empty winning recipe = %+v, missing = %+v", rule, got.missing)
	}
	if _, ok := got.rule("false-rule"); ok {
		t.Fatal("a define or continued comment became a rule")
	}
	if _, ok := got.conditional("missing"); ok {
		t.Fatal("an absent conditional target appeared")
	}
	if _, ok := got.conditionalVar("missing"); ok {
		t.Fatal("an absent conditional variable appeared")
	}
}

func TestMakeReaderBoundsExpansionWork(t *testing.T) {
	t.Parallel()
	body := []string{"V0=" + strings.Repeat("x", 1024) + "\n"}
	for i := 1; i <= 14; i++ {
		body = append(body, fmt.Sprintf("V%d=$(V%d)$(V%d)\n", i, i-1, i-1))
	}
	body = append(body, "ci: $(V14)\n")
	got, err := readMakefile(makeInput(map[string]string{"Makefile": strings.Join(body, "")}), "Makefile")
	if err != nil {
		t.Fatal(err)
	}
	rule, ok := got.rule("ci")
	if !ok || !slices.Equal(rule.prerequisites, []string{"$(V14)"}) {
		t.Fatal("expansion exceeded the snapshot read budget instead of preserving its token")
	}
}
