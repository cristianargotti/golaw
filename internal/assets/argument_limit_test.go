package assets_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/runner"
)

func TestLintAssetLimitsArgumentsToSeven(t *testing.T) {
	t.Parallel()
	for _, caps := range [][]string{{"go"}, {"go", "hexagonal"}, {"api", "db", "go", "hexagonal"}} {
		context := assetContext()
		context.Capabilities = caps
		var config map[string]any
		if err := yaml.Unmarshal(renderedAsset(t, ".golangci.yml", context).Data, &config); err != nil {
			t.Fatal(err)
		}
		revive := yamlMap(t, yamlMap(t, yamlMap(t, config, "linters"), "settings"), "revive")
		names := make([]string, 0, 48)
		limited := false
		for _, rule := range anyList(t, revive["rules"]) {
			name, arguments := reviveRule(t, rule)
			names = append(names, name)
			limited = limited || name == "argument-limit" && reflect.DeepEqual(arguments, []any{7})
		}
		if !limited {
			t.Errorf("the lint asset for %v lacks revive argument-limit 7: %v", caps, names)
		}
		if got := strings.Join(names, " "); !strings.Contains(got, "file-length-limit argument-limit function-length") {
			t.Errorf("argument-limit is not between file-length-limit and function-length: %s", got)
		}
	}
}

// reviveRule reads the name and the arguments of one revive rule; the
// type assertions are checked because errcheck holds tests to them too.
func reviveRule(t *testing.T, rule any) (string, any) {
	t.Helper()
	fields, ok := rule.(map[string]any)
	if !ok {
		t.Fatalf("a revive rule is not a mapping: %v", rule)
	}
	name, named := fields["name"].(string)
	if !named {
		t.Fatalf("a revive rule has no name: %v", rule)
	}
	return name, fields["arguments"]
}

// TestRenderedLintAssetCountsParameters runs the pinned golangci-lint with
// revive alone over a module beside the rendered asset: eight parameters
// are the red shape of go:S107, seven the near miss.
func TestRenderedLintAssetCountsParameters(t *testing.T) {
	t.Parallel()
	tool, ok := runner.LookupTool("golangci-lint")
	if !ok {
		t.Fatal("golangci-lint is not a pinned tool")
	}
	env := append((&exec.Cmd{}).Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOWORK=off", "GOFLAGS=-mod=mod")
	linter, err := runner.Verify(t.Context(), tool, env)
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("CI installs golangci-lint %s before the tests: %v", tool.Version, err)
		}
		t.Skipf("golangci-lint %s is not on PATH, which make ci installs: %v", tool.Version, err)
	}
	for _, tc := range []struct {
		name   string
		params []string
		red    bool
	}{
		{"red-eight-parameters", []string{"a", "b", "c", "d", "e", "f", "g", "h"}, true},
		{"near-seven-parameters", []string{"a", "b", "c", "d", "e", "f", "g"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := parameterModule(t, tc.params)
			out, runErr := runner.Run(t.Context(), runner.Command{
				Name: linter, Args: []string{"run", "--allow-parallel-runners", "--enable-only=revive", "./..."}, Dir: root, Env: env,
			})
			if runErr != nil {
				t.Fatal(runErr)
			}
			flagged := strings.Contains(string(out.Stdout), "argument-limit")
			if flagged != tc.red || (out.Code != 0) != tc.red {
				t.Fatalf("golangci-lint = %d: %s%s", out.Code, out.Stdout, out.Stderr)
			}
		})
	}
}

// parameterModule writes the core templates as init renders them and one
// package whose exported function takes the parameters.
func parameterModule(t *testing.T, params []string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	files := renderedRepository(t, []string{"go"})
	files["main.go"] = []byte("// Command service is the module the core templates are rendered for.\npackage main\n\nfunc main() {}\n")
	files["calc/calc.go"] = []byte("// Package calc adds integers.\npackage calc\n\n// Sum adds its arguments.\nfunc Sum(" +
		strings.Join(params, ", ") + " int) int {\n\treturn " + strings.Join(params, " + ") + "\n}\n")
	for relative, content := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
