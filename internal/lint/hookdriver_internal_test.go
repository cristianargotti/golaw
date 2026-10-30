package lint

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

func TestHookDriverPicksTheFastToolOfALanguage(t *testing.T) {
	t.Parallel()
	for language, tool := range map[surface.Language]string{surface.Python: "ruff", surface.Dockerfile: "hadolint", surface.Shell: "shellcheck"} {
		d, ok := HookDriver(language)
		if !ok || d.Tool != tool || d.HookArgs == nil {
			t.Errorf("%s = %+v, %t, want %s with hook arguments", language, d, ok, tool)
		}
	}
	for _, language := range []surface.Language{surface.JavaScript, surface.TypeScript, surface.JSX, surface.TSX, surface.HTML, surface.CSS, surface.CloudFormation, surface.YAML} {
		if d, ok := HookDriver(language); ok {
			t.Errorf("%s runs %s in the hook; M6.6.1 gives it the native checks only", language, d.Tool)
		}
	}
}

func TestHookShellReadsTheShebangAndTheExtension(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path, content, shell string
		ok                   bool
	}{
		{"scripts/a.sh", "#!/bin/sh\necho\n", "sh", true},
		{"scripts/a", "#!/usr/bin/env bash\necho\n", "bash", true},
		{"scripts/a", "#!/usr/bin/env -S dash -e\necho\n", "dash", true},
		{"scripts/a.sh", "#!/bin/ksh\necho\n", "ksh", true},
		{"scripts/a.bash", "echo\n", "bash", true},
		{"scripts/a.sh", "echo\n", "sh", true},
		{"scripts/a", "#!/bin/zsh\necho\n", "", false},
		{"scripts/a.sh", "#!/usr/bin/env fish\necho\n", "", false},
	}
	for _, tt := range tests {
		if shell, ok := HookShell(tt.path, []byte(tt.content)); shell != tt.shell || ok != tt.ok {
			t.Errorf("%s %q = %q, %t, want %q, %t", tt.path, tt.content, shell, ok, tt.shell, tt.ok)
		}
	}
}

func TestRunHookSkipsAndNamesAMissingToolWithoutAProcess(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	env := HookEnv{Resolver: toolchain.Resolver{Dir: dir, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}, ToolsDir: dir, Set: model.SurfaceParity}
	if out := RunHook(t.Context(), env, HookFile{Path: "views/a.js", Content: []byte("x()\n"), Language: surface.JavaScript}); !out.Skipped {
		t.Errorf("a script = %+v, want skipped", out)
	}
	if out := RunHook(t.Context(), env, HookFile{Path: "scripts/a", Content: []byte("#!/bin/zsh\n"), Language: surface.Shell}); !out.Skipped {
		t.Errorf("a zsh script = %+v, want skipped", out)
	}
	out := RunHook(t.Context(), env, HookFile{Path: "a.py", Content: []byte("x = 1\n"), Language: surface.Python})
	if !out.Missing || out.Tool.Name != "ruff" || len(out.Issues) != 0 {
		t.Errorf("no receipt = %+v, want ruff missing", out)
	}
}

func TestHookRequestPipesTheBytesFromTheToolDirectory(t *testing.T) {
	t.Parallel()
	d, _ := HookDriver(surface.Python)
	tool, _ := toolchain.Lookup("ruff")
	dir := t.TempDir()
	env := HookEnv{ToolsDir: dir, Kit: "0.3.0", Set: model.SurfaceParity, Env: []string{"PATH=/usr/bin"}}
	file := HookFile{Path: "test/deploy/check_install.py", Content: []byte("x = 1\n"), Language: surface.Python}
	req, err := hookRequest(env, d, tool, "/opt/ruff", file)
	if err != nil {
		t.Fatal(err)
	}
	if req.Path != "/opt/ruff" || string(req.Stdin) != "x = 1\n" || req.Dir != dir || req.Timeout != toolchain.HookToolBudget || len(req.Env) != 1 {
		t.Fatalf("request = %+v", req)
	}
	if want := d.HookArgs(HookInput{Path: file.Path, ConfigDir: toolchain.ConfigDir(dir, "0.3.0", model.SurfaceParity)}); fmt.Sprint(req.Args) != fmt.Sprint(want) {
		t.Fatalf("args = %q, want %q", req.Args, want)
	}
}

func TestHookResultReadsTheToolAnswer(t *testing.T) {
	t.Parallel()
	d, _ := HookDriver(surface.Python)
	tool, _ := toolchain.Lookup("ruff")
	root := t.TempDir()
	file := HookFile{Path: "test/deploy/check_install.py", Language: surface.Python}
	name, err := json.Marshal(filepath.Join(root, "test", "deploy", "check_install.py"))
	if err != nil {
		t.Fatal(err)
	}
	answer := `[{"cell":null,"code":"S105","end_location":{"column":20,"row":3},"filename":` + string(name) +
		`,"fix":null,"location":{"column":9,"row":3},"message":"Possible hardcoded secret","noqa_row":3,"url":"https://docs.astral.sh/ruff/rules/hardcoded-password-string"}]`
	tests := []struct {
		name string
		run  toolchain.Execution
		err  error
		want HookOutcome
	}{
		{"a killed tool", toolchain.Execution{Killed: true}, nil, HookOutcome{Tool: tool, Killed: true}},
		{"a tool that cannot start", toolchain.Execution{}, runner.ErrStart, HookOutcome{Tool: tool, Missing: true}},
		{"an answer golaw cannot read", toolchain.Execution{Output: runner.Output{Stdout: []byte("not json"), Code: 2}}, nil, HookOutcome{Tool: tool, Killed: true}},
	}
	for _, tt := range tests {
		got := hookResult(tool, d, file, root, tt.run, tt.err)
		if got.Killed != tt.want.Killed || got.Missing != tt.want.Missing || got.Tool.Name != "ruff" || len(got.Issues) != 0 {
			t.Errorf("%s = %+v", tt.name, got)
		}
	}
	got := hookResult(tool, d, file, root, toolchain.Execution{Output: runner.Output{Stdout: []byte(answer), Code: 1}}, nil)
	if len(got.Issues) != 1 || got.Issues[0].Path != file.Path || got.Issues[0].Rule != "S105" || got.Issues[0].Line != 3 || got.Killed || got.Missing {
		t.Fatalf("a finding = %+v", got)
	}
}
