package rung

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/runner"
)

// TestLintLeavesAnUnrequiredRuleguardModuleToThePipeline holds the lint
// step to what a commit introduces: a go.mod without the rules module of
// the kit stops golangci-lint before it judges a line, which no staged
// change caused, so the step names the repair and lets the commit go, while
// any other failure of the tool still refuses it.
func TestLintLeavesAnUnrequiredRuleguardModuleToThePipeline(t *testing.T) {
	t.Parallel()
	const stopped = "level=error msg=\"Running error: can't run linter goanalysis_metalinter\\ngocritic: ruleguard init error: typechecker error: " +
		"/work/scripts/lint/rules.go:9:8: could not import github.com/quasilyte/go-ruleguard/dsl (can't find import)\"\n"
	out := linted(1, runner.Output{Code: 3, Stderr: []byte(stopped)}, nil)
	text := strings.Join(out.Lines, "\n")
	if out.Code != codeClean || !strings.Contains(text, "lint: left to the pipeline") ||
		!strings.Contains(text, "run go get github.com/quasilyte/go-ruleguard/dsl@v0.3.23 and then go mod tidy") {
		t.Errorf("an unrequired rules module = %+v", out)
	}
	other := linted(1, runner.Output{Code: 3, Stderr: []byte("level=error msg=\"no go files to analyze\"\n")}, nil)
	if other.Code != codeFailure || !strings.Contains(strings.Join(other.Lines, "\n"), "golangci-lint failed with exit status 3") {
		t.Errorf("another golangci-lint failure = %+v", other)
	}
}

// TestLintNamesTheRulesModuleTheTemplateRequires keeps the repair the step
// prints at the version init proposes for go.mod.
func TestLintNamesTheRulesModuleTheTemplateRequires(t *testing.T) {
	t.Parallel()
	template, err := os.ReadFile(filepath.Join("..", "..", "templates", "go.mod.tmpl"))
	if err != nil {
		t.Fatal(err)
	}
	module, version, _ := strings.Cut(ruleguardModule, "@")
	if !strings.Contains(string(template), "require "+module+" "+version+"\n") {
		t.Errorf("the lint step repairs with %s, which templates/go.mod.tmpl does not require", ruleguardModule)
	}
}
