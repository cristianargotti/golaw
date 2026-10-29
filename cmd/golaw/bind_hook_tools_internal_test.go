package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/hometest"
	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/proof"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

const checkInstall = "test/deploy/check_install.py"

// installFake puts a fake of the pinned tool where setup --surface puts the
// real one, with its receipt, and answers the directory that holds it.
func installFake(t *testing.T, toolsDir, name string, b runnertest.Behavior) string {
	t.Helper()
	tool, ok := toolchain.Lookup(name)
	if !ok {
		t.Fatalf("%s is no pinned tool", name)
	}
	dir := filepath.Dir(toolchain.BinPath(toolsDir, tool, runtime.GOOS, runtime.GOARCH))
	b.Probe, b.Version = tool.Probe, tool.Version
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	// The first start of a fresh copy waits for the system's check of a new
	// executable, which under load passes the tool budget the test judges;
	// one quiet start first, then the behavior with a fresh call record.
	warm := runnertest.Install(t, dir, name, runnertest.Behavior{})
	if _, err := runner.Run(t.Context(), runner.Command{Name: warm}); err != nil {
		t.Fatal(err)
	}
	path := runnertest.Install(t, dir, name, b)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	receipt := toolchain.Receipt{Name: name, Version: tool.Version, Path: path, SHA256: hex.EncodeToString(sum[:]), Size: info.Size(), ModTime: info.ModTime()}
	if err := toolchain.WriteReceipt(toolsDir, receipt); err != nil {
		t.Fatal(err)
	}
	return dir
}

// toolBinding is the hook binding of a temporary home: its tool directory
// and nothing of the developer's.
func toolBinding(t *testing.T) (hookBinding, hometest.Home) {
	t.Helper()
	home := hometest.New(t)
	b := binding{goos: runtime.GOOS, env: home.Getenv, environ: home.Environ}
	return hookBinding{binding: b, dirs: policy.Dirs{State: home.State, Config: home.Config, Tools: home.Tools()}}, home
}

func ruffFinding(t *testing.T, toolsDir string) string {
	t.Helper()
	name, err := json.Marshal(filepath.Join(toolsDir, filepath.FromSlash(checkInstall)))
	if err != nil {
		t.Fatal(err)
	}
	return `[{"cell":null,"code":"S105","end_location":{"column":20,"row":3},"filename":` + string(name) +
		`,"fix":null,"location":{"column":9,"row":3},"message":"Possible hardcoded secret","noqa_row":3,"url":"https://docs.astral.sh/ruff/rules/hardcoded-password-string"}]`
}

func TestHookToolsRunRuffOnTheBytesWritten(t *testing.T) {
	t.Parallel()
	h, home := toolBinding(t)
	dir := installFake(t, home.Tools(), "ruff", runnertest.Behavior{Stdout: ruffFinding(t, home.Tools()), Code: 1, Stdin: true})
	content := []byte("import os\n\nvalue = os.environ[\"X\"]\n")
	report := h.tools(t.Context(), hook.Repository{Root: t.TempDir()}, proof.Policy(), []hook.ToolFile{{Path: checkInstall, Content: content, Language: surface.Python}})
	if len(report.Findings) != 1 || len(report.Missing) != 0 || len(report.Killed) != 0 {
		t.Fatalf("report = %+v", report)
	}
	f := report.Findings[0]
	if f.Check != "lint.ruff" || f.Rule != "ruff/S105" || f.Path != checkInstall || f.Line != 3 || f.Code != "s105" || f.Sonar != "python:S2068" || f.Severity != model.SeverityFail {
		t.Fatalf("finding = %+v", f)
	}
	if inputs := runnertest.Inputs(t, dir, "ruff"); len(inputs) != 1 || !bytes.Equal(inputs[0], content) {
		t.Fatalf("stdin = %q, want the bytes written", inputs)
	}
	calls := runnertest.Calls(t, dir, "ruff")
	if len(calls) != 1 || !slices.Contains(calls[0], "--config") || slices.Contains(calls[0], "--isolated") || !slices.Contains(calls[0], "--stdin-filename") || !slices.Contains(calls[0], checkInstall) || calls[0][len(calls[0])-1] != "-" {
		t.Fatalf("calls = %q, want one ruff check on stdin, no version probe", calls)
	}
}

func TestHookToolsNameAMissingToolAndSkipTheSlowLanguages(t *testing.T) {
	t.Parallel()
	h, _ := toolBinding(t)
	report := h.tools(t.Context(), hook.Repository{Root: t.TempDir()}, proof.Policy(), []hook.ToolFile{
		{Path: checkInstall, Content: []byte("x = 1\n"), Language: surface.Python},
		{Path: "test/deploy/other.py", Content: []byte("y = 1\n"), Language: surface.Python},
		{Path: "views/panel.js", Content: []byte("run();\n"), Language: surface.JavaScript},
	})
	tool, _ := toolchain.Lookup("ruff")
	agent, person := toolchain.MissingNotice(tool, checkInstall, model.DefaultLanguage)
	if !strings.HasPrefix(person, "ruff 0.17.1 não está instalado no diretório de ferramentas do kit:") {
		t.Fatalf("person = %q, want the pt-BR sentence of the configured language", person)
	}
	if !slices.Equal(report.Missing, []string{agent}) || !slices.Equal(report.Person, []string{person}) || len(report.Findings) != 0 || len(report.Killed) != 0 {
		t.Fatalf("report = %+v, want one notice for ruff and nothing for the script", report)
	}
}

func TestHookToolsKillASlowToolWithinTheEventBudget(t *testing.T) {
	t.Parallel()
	h, home := toolBinding(t)
	installFake(t, home.Tools(), "shellcheck", runnertest.Behavior{Stdout: "{\"comments\":[]}", Delay: "5s"})
	files := []hook.ToolFile{
		{Path: "scripts/a.sh", Content: []byte("#!/bin/sh\necho a\n"), Language: surface.Shell},
		{Path: "scripts/b.sh", Content: []byte("#!/bin/sh\necho b\n"), Language: surface.Shell},
		{Path: "scripts/c.sh", Content: []byte("#!/bin/sh\necho c\n"), Language: surface.Shell},
	}
	start := time.Now()
	report := h.tools(t.Context(), hook.Repository{Root: t.TempDir()}, proof.Policy(), files)
	if elapsed := time.Since(start); elapsed > toolchain.HookEventBudget+time.Second {
		t.Fatalf("the event took %s, over its budget of %s", elapsed, toolchain.HookEventBudget)
	}
	if !slices.Equal(report.Killed, []string{"scripts/a.sh", "scripts/b.sh", "scripts/c.sh"}) || len(report.Findings) != 0 {
		t.Fatalf("report = %+v, want the three files given up", report)
	}
}

func TestHookDependenciesBindTheFastTools(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	deps, err := b.hookDependencies(agentUnderTest(), now())
	if err != nil {
		t.Fatal(err)
	}
	if deps.Tools == nil {
		t.Fatal("the hook runs no fast tool")
	}
}
