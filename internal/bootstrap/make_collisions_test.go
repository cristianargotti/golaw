package bootstrap_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/bootstrap"
)

// birthMakefile is the shape of the Makefile a scaffolded service is born
// with: conditionals over the goals, a $(shell) inside a recipe, and lint
// and test targets that golaw.mk defines too.
const birthMakefile = "LOG_LEVEL := error\nENVIRONMENT :=\n\n" +
	"ifneq (,$(filter log_debug,$(MAKECMDGOALS)))\n\tLOG_LEVEL := debug\nendif\n\n" +
	"ifneq (,$(filter qa,$(MAKECMDGOALS)))\n\tENVIRONMENT := qa\nelse ifneq (,$(filter live,$(MAKECMDGOALS)))\n" +
	"\tENVIRONMENT := live\nelse\n\tENVIRONMENT := dev\nendif\n\n" +
	".PHONY: install-deps lint lint-fix test qa live log_debug\n\n" +
	"install-deps:\n\t@command -v golangci-lint >/dev/null 2>&1 || \\\n" +
	"\t\tGOBIN=$(shell go env GOPATH)/bin go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest\n\n" +
	"lint:\n\t@golangci-lint run -v\n\nlint-fix:\n\t@golangci-lint run -v --fix\n\n" +
	"test:\n\t@docker compose run --rm test\n\nqa:\n\t@:\n\nlive:\n\t@:\n\nlog_debug:\n\t@:\n"

// kitMakefile is the shape of golaw.mk the collisions are read against.
const kitMakefile = "export GOTOOLCHAIN=local\n\n.PHONY: lint test vet\n\nlint:\n\tgolangci-lint run ./...\n\n" +
	"test:\n\tgo test ./...\n\nvet:\n\tgo vet ./...\n"

func TestMakeCollisionsNamesTargetsBothFilesDefine(t *testing.T) {
	t.Parallel()
	renamed := strings.ReplaceAll(strings.ReplaceAll(birthMakefile, "\nlint:", "\nlint-template:"), "\ntest:", "\ntest-compose:")
	for _, tc := range []struct {
		name, makefile string
		want           []bootstrap.Collision
	}{
		{"red birth", birthMakefile, []bootstrap.Collision{{Target: "lint", Line: 22}, {Target: "test", Line: 28}}},
		{"red double colon with two targets", "all vet:: dep\n\techo\n", []bootstrap.Collision{{Target: "vet", Line: 1}}},
		{"red continued rule line", "lint \\\n  check:\n\techo\n", []bootstrap.Collision{{Target: "lint", Line: 1}}},
		{"red target defined twice", "lint:\n\techo\nlint: dep\n", []bootstrap.Collision{{Target: "lint", Line: 1}}},
		{"near renamed targets", renamed, nil},
		{"near assignments, patterns and references", "lint := golangci-lint\ntest ::= x\n%.o: %.c\n\tcc\n$(LINT): dep\n.PHONY: lint\n# lint:\n", nil},
		{"near define block", "define lint\nlint:\n\techo\nendef\n", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := bootstrap.MakeCollisions([]byte(tc.makefile), []byte(kitMakefile)); !slices.Equal(got, tc.want) {
				t.Fatalf("collisions = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestTemplatePlanNamesMakefileCollisions(t *testing.T) {
	t.Parallel()
	rendered := []assets.Rendered{
		templateFile("Makefile", "include golaw.mk\n", assets.OwnedMerged),
		templateFile("golaw.mk", kitMakefile, assets.OwnedByKit),
	}
	root := t.TempDir()
	writeBefore(t, filepath.Join(root, "Makefile"), birthMakefile)
	plan, err := bootstrap.PlanTemplates(root, rendered)
	if err != nil {
		t.Fatal(err)
	}
	want := []bootstrap.Collision{{Target: "lint", Line: 22}, {Target: "test", Line: 28}}
	if !slices.Equal(plan.Collisions, want) {
		t.Fatalf("plan collisions = %+v", plan.Collisions)
	}
	for _, tc := range []struct{ language, text string }{
		{"", "lint (Makefile:22), test (Makefile:28). Renomeie ou remova cada alvo no Makefile antes de aceitar a proposta do include."},
		{"en", "lint (Makefile:22), test (Makefile:28). Rename or remove each target in the Makefile before you accept the include proposal."},
	} {
		plan.Language = tc.language
		var out bytes.Buffer
		if diffErr := bootstrap.WriteTemplateDiff(&out, plan); diffErr != nil {
			t.Fatal(diffErr)
		}
		if !strings.Contains(out.String(), tc.text) {
			t.Fatalf("diff of %q lacks %q:\n%s", tc.language, tc.text, out.String())
		}
	}
	fresh, err := bootstrap.PlanTemplates(t.TempDir(), rendered)
	if err != nil || len(fresh.Collisions) != 0 {
		t.Fatalf("a repository without a Makefile has collisions: %+v, %v", fresh.Collisions, err)
	}
}

func TestMakeCollisionsKeepsRuleBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, makefile string
		want           []bootstrap.Collision
	}{
		{"target assignments", "lint: FLAGS := value\ntest: export X = value\nvet: private Y ?= value\n", nil},
		{"nested define", "define OUTER\ndefine INNER\nendef\nlint:\nendef\n", nil},
		{"modified define", "override define RULE\nlint:\nendef\nexport define OTHER\ntest:\nendef\n", nil},
		{"modifier inside define", "define RULE\nexport endef\nlint:\nendef\n", nil},
		{"recipe inside define", "define RULE\n\tendef\nlint:\nendef\n", nil},
		{"continued recipe", "all:\n\tprintf \\\nlint:\n", nil},
		{"continued comment", "# ignored \\\nlint:\n", nil},
		{"custom recipe prefix", ".RECIPEPREFIX = l\nall:\nlint:\n", nil},
		{"CRLF continuation", "# first\r\nlint \\\r\n check:\r\n\techo\r\n", []bootstrap.Collision{{Target: "lint", Line: 2}}},
		{"inline recipe assignment", "lint: ; VALUE=x command\n", []bootstrap.Collision{{Target: "lint", Line: 1}}},
		{"rule after define", "define RULE\n.RECIPEPREFIX = l\nendef\nlint:\n\techo\n", []bootstrap.Collision{{Target: "lint", Line: 4}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := bootstrap.MakeCollisions([]byte(tc.makefile), []byte(kitMakefile))
			if !slices.Equal(got, tc.want) {
				t.Fatalf("collisions = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestTemplateCollisionsRequireBothRenderedInputs(t *testing.T) {
	t.Parallel()
	for _, file := range []assets.Rendered{
		templateFile("Makefile", "include golaw.mk\n", assets.OwnedMerged),
		templateFile("golaw.mk", kitMakefile, assets.OwnedByKit),
	} {
		t.Run(file.Path, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeBefore(t, filepath.Join(root, "Makefile"), birthMakefile)
			plan, err := bootstrap.PlanTemplates(root, []assets.Rendered{file})
			if err != nil || len(plan.Collisions) != 0 {
				t.Fatalf("incomplete inputs have collisions: %+v, %v", plan.Collisions, err)
			}
		})
	}
}

func TestTemplateCollisionNoticeFailureWritesNothing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "Makefile")
	writeBefore(t, path, birthMakefile)
	plan, err := bootstrap.PlanTemplates(root, []assets.Rendered{
		templateFile("Makefile", "include golaw.mk\n", assets.OwnedMerged),
		templateFile("golaw.mk", kitMakefile, assets.OwnedByKit),
	})
	if err != nil {
		t.Fatal(err)
	}
	writer := &templateMutationWriter{fail: true, calls: 1}
	err = bootstrap.ApplyTemplates(root, plan, writer)
	if !errors.Is(err, io.ErrClosedPipe) || err.Error() != "bootstrap: write collisions: "+io.ErrClosedPipe.Error() {
		t.Fatalf("collision notice error = %v", err)
	}
	requireTemplateAbsent(t, root, "golaw.mk")
	current, err := os.ReadFile(path)
	if err != nil || string(current) != birthMakefile {
		t.Fatalf("Makefile changed: %q, %v", current, err)
	}
}

func TestMakeCollisionsRequiresConflictingRules(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, makefile, included string
		want                     []bootstrap.Collision
	}{
		{"red later recipe", "lint: dep\nlint:\n\techo\n", kitMakefile, []bootstrap.Collision{{Target: "lint", Line: 2}}},
		{"red recipe after gaps", "lint:\n\n# comment\n\techo\n", kitMakefile, []bootstrap.Collision{{Target: "lint", Line: 1}}},
		{"red inline recipes", "lint: dep; echo\n", "lint:; check\n", []bootstrap.Collision{{Target: "lint", Line: 1}}},
		{"red double colon without recipe", "lint:: dep\n", "lint: kit\n", []bootstrap.Collision{{Target: "lint", Line: 1}}},
		{"red included double colon", "lint: dep\n", "lint:: kit\n", []bootstrap.Collision{{Target: "lint", Line: 1}}},
		{"red first colon conflict", "lint: dep\nlint:: dep\nlint:; echo\n", kitMakefile, []bootstrap.Collision{{Target: "lint", Line: 2}}},
		{"red first recipe conflict", "lint: dep\nlint:; echo\nlint:: dep\n", kitMakefile, []bootstrap.Collision{{Target: "lint", Line: 2}}},
		{
			"red conflict order", "lint: dep\nvet:; echo\nlint:; echo\n", kitMakefile,
			[]bootstrap.Collision{{Target: "vet", Line: 2}, {Target: "lint", Line: 3}},
		},
		{"red ci recipes", "ci:\n\techo\n", "ci: lint test\n\tcheck\n", []bootstrap.Collision{{Target: "ci", Line: 1}}},
		{"red included later recipe", "ci:; echo\n", "ci: lint test\nci:; check\n", []bootstrap.Collision{{Target: "ci", Line: 1}}},
		{"near prerequisites", "lint: dep\ntest: generate\n", kitMakefile, nil},
		{"near conditional recipe", "lint:\nifdef CI\n\techo\nendif\n", kitMakefile, nil},
		{"near following rule recipe", "lint: dep\nother:\n\techo\n", kitMakefile, nil},
		{"near comment semicolon", "lint: dep # ; echo\n", kitMakefile, nil},
		{"near ci prerequisites", "ci: local\n", "ci: lint test\n", nil},
		{"near included ci prerequisites", "ci:\n\techo\n", "ci: lint test\n", nil},
		{"near included conditional recipe", "ci:; echo\n", "ci: lint test\nifdef CI\n\tcheck\nendif\n", nil},
		{"near matching double colons", "ci:: local\n", "ci:: lint test\n", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := bootstrap.MakeCollisions([]byte(tc.makefile), []byte(tc.included))
			if !slices.Equal(got, tc.want) {
				t.Fatalf("collisions = %+v, want %+v", got, tc.want)
			}
		})
	}
}
