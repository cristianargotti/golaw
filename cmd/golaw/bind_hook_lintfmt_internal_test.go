package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/proof"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/toolchain"
	"github.com/cristianargotti/golaw/internal/toolchaintest"
)

const (
	lintFmtPath   = "internal/application/growth/policy.go"
	lintFmtInput  = "package growth\n\nimport \"strings\"\n\nfunc label( ) string {return \"x\"}\n"
	inProcessDone = "package growth\n\nfunc label() string { return \"x\" }\n"
	linterDone    = "package growth\n\nfunc label() string {\n\treturn \"x\"\n}\n"
)

// installLinter puts a warmed fake of the pinned golangci-lint where setup
// --gen builds the real one, and answers its directory.
func installLinter(t *testing.T, h hookBinding, b runnertest.Behavior) string {
	t.Helper()
	rt, ok := runner.LookupTool("golangci-lint")
	if !ok {
		t.Fatal("golangci-lint is no pinned tool")
	}
	tool := toolchain.GoTool(rt)
	warm := toolchaintest.Install(t, h.dirs.Tools, tool, runtime.GOOS, runtime.GOARCH, runnertest.Behavior{})
	if _, err := runner.Run(t.Context(), runner.Command{Name: warm}); err != nil {
		t.Fatal(err)
	}
	return filepath.Dir(toolchaintest.Install(t, h.dirs.Tools, tool, runtime.GOOS, runtime.GOARCH, b))
}

func lintFmtRun(t *testing.T, h hookBinding) (string, []fixedFile) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, filepath.FromSlash(lintFmtPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(lintFmtInput), 0o600); err != nil {
		t.Fatal(err)
	}
	snap := proof.NewTree(map[string][]byte{"go.mod": []byte("module example.com/slice\n\ngo 1.27\n"), lintFmtPath: []byte(lintFmtInput)})
	fixed := fixFiles(t.Context(), root, "example.com/slice", snap, []string{lintFmtPath}, h.lintFmt(root))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data), fixed
}

func TestTheLinterFormatterRunsOnTheBytesTheInProcessFixesLeft(t *testing.T) {
	t.Parallel()
	h := regenBinding(t)
	dir := installLinter(t, h, runnertest.Behavior{Stdout: linterDone, Stdin: true})
	got, fixed := lintFmtRun(t, h)
	if got != linterDone || len(fixed) != 1 || !slices.Equal(fixed[0].parts, []string{"go/format", "imports fixed", "golangci-lint fmt"}) {
		t.Fatalf("file =\n%s\nfixed = %+v", got, fixed)
	}
	inputs := runnertest.Inputs(t, dir, "golangci-lint")
	if len(inputs) == 0 || !bytes.Equal(inputs[len(inputs)-1], []byte(inProcessDone)) {
		t.Fatalf("stdin = %q", inputs)
	}
	calls := runnertest.Calls(t, dir, "golangci-lint")
	if !slices.Equal(calls[len(calls)-1], []string{"fmt", "--stdin"}) {
		t.Fatalf("calls = %q", calls)
	}
}

func TestAFailingOrAbsentLinterLeavesTheInProcessFixes(t *testing.T) {
	t.Parallel()
	for name, behavior := range map[string]*runnertest.Behavior{
		"near: a linter that fails":        {Code: 3, Stderr: "no go files\n"},
		"near: a linter past the budget":   {Delay: "2s"},
		"near: no linter setup ever built": nil,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := regenBinding(t)
			if behavior != nil {
				installLinter(t, h, *behavior)
			} else if h.lintFmt(t.TempDir()) != nil {
				t.Fatal("a stage without a receipted linter")
			}
			got, fixed := lintFmtRun(t, h)
			if got != inProcessDone || len(fixed) != 1 || slices.Contains(fixed[0].parts, "golangci-lint fmt") {
				t.Fatalf("file =\n%s\nfixed = %+v", got, fixed)
			}
		})
	}
}
