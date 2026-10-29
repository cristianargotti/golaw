package rung_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cristianargotti/golaw/internal/hometest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/rung"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

const staged = "test/deploy/check_install.py"

// installTool puts a fake of the pinned tool where setup --surface puts the
// real one, with its receipt, and answers its directory.
func installTool(t *testing.T, toolsDir, name string, b runnertest.Behavior) string {
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
	// executable, which under load passes the tool budget the rung keeps;
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
	if err := toolchain.WriteReceipt(toolsDir, toolchain.Receipt{Name: name, Version: tool.Version, Path: path, SHA256: hex.EncodeToString(sum[:]), Size: info.Size(), ModTime: info.ModTime()}); err != nil {
		t.Fatal(err)
	}
	return dir
}

// ruffAt is the answer of ruff to one finding on a line of the staged file.
func ruffAt(t *testing.T, toolsDir string, line int) runnertest.Behavior {
	t.Helper()
	name, err := json.Marshal(filepath.Join(toolsDir, filepath.FromSlash(staged)))
	if err != nil {
		t.Fatal(err)
	}
	row, err := json.Marshal(line)
	if err != nil {
		t.Fatal(err)
	}
	return runnertest.Behavior{Code: 1, Stdout: `[{"cell":null,"code":"S105","end_location":{"column":20,"row":` + string(row) + `},"filename":` + string(name) +
		`,"fix":null,"location":{"column":9,"row":` + string(row) + `},"message":"Possible hardcoded secret","noqa_row":` + string(row) + `,"url":"https://docs.astral.sh/ruff/rules/hardcoded-password-string"}]`}
}

const checkSource = "import os\n\nvalue = os.environ[\"X\"]\n"

//nolint:paralleltest // the fake gitleaks is found through PATH
func TestPreCommitRefusesWhatTheFastLinterFindsOnTheLinesItAdds(t *testing.T) {
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks})
	home := hometest.New(t)
	dir := installTool(t, home.Tools(), "ruff", ruffAt(t, home.Tools(), 3))
	r := armedRepo(t, armedPolicy)
	r.Stage(map[string]string{staged: checkSource})
	out := rung.RunPreCommit(t.Context(), deps(t, r, "XDG_DATA_HOME="+home.Data))
	if out.Code != 1 || !contains(out, "golaw pre-commit: surface: "+staged+":3 ruff/S105 (python:S2068) Possible hardcoded secret") {
		t.Fatalf("outcome = %+v", out)
	}
	if calls := runnertest.Calls(t, dir, "ruff"); len(calls) != 1 {
		t.Fatalf("ruff calls = %q, want one on the staged bytes", calls)
	}
}

//nolint:paralleltest // the fake gitleaks is found through PATH
func TestPreCommitLeavesALegacyLineToThePipeline(t *testing.T) {
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks})
	home := hometest.New(t)
	installTool(t, home.Tools(), "ruff", ruffAt(t, home.Tools(), 3))
	r := armedRepo(t, armedPolicy)
	r.Commit("feat: add the install check", map[string]string{staged: checkSource})
	r.Stage(map[string]string{staged: checkSource + "other = 1\n"})
	out := rung.RunPreCommit(t.Context(), deps(t, r, "XDG_DATA_HOME="+home.Data))
	if out.Code != 0 || !contains(out, "golaw pre-commit: surface: "+staged+": 1 finding on lines this commit does not touch, left to the pipeline") {
		t.Fatalf("outcome = %+v", out)
	}
}

//nolint:paralleltest // the fake gitleaks is found through PATH
func TestPreCommitPassesWithTheNoticeOfAMissingTool(t *testing.T) {
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks})
	home := hometest.New(t)
	r := armedRepo(t, armedPolicy)
	r.Stage(map[string]string{staged: checkSource})
	out := rung.RunPreCommit(t.Context(), deps(t, r, "XDG_DATA_HOME="+home.Data))
	tool, _ := toolchain.Lookup("ruff")
	_, person := toolchain.MissingNotice(tool, staged, model.DefaultLanguage)
	if out.Code != 0 || !contains(out, "golaw pre-commit: surface: "+person) {
		t.Fatalf("outcome = %+v, want a pass with the person's pt-BR notice", out)
	}
}

//nolint:paralleltest // the fake gitleaks is found through PATH
func TestPreCommitPassesAShellFindingUnderParity(t *testing.T) {
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks})
	home := hometest.New(t)
	installTool(t, home.Tools(), "shellcheck", runnertest.Behavior{Code: 1, Stdout: `{"comments":[{"file":"-","line":2,"endLine":2,"column":6,"endColumn":8,"level":"info","code":2086,"message":"Double quote to prevent globbing and word splitting.","fix":null}]}`})
	const script = "#!/bin/sh\necho $1\n"
	const line = "golaw pre-commit: surface: scripts/deploy.sh:2 shellcheck/SC2086"
	parity := armedRepo(t, armedPolicy+"surface: {rules: parity}\n")
	parity.Stage(map[string]string{"scripts/deploy.sh": script})
	if out := rung.RunPreCommit(t.Context(), deps(t, parity, "XDG_DATA_HOME="+home.Data)); out.Code != 0 || !contains(out, line) {
		t.Fatalf("parity = %+v, want a pass that names the finding (the parity set has no shell rule)", out)
	}
	strict := armedRepo(t, armedPolicy+"surface: {rules: strict}\n")
	strict.Stage(map[string]string{"scripts/deploy.sh": script})
	if out := rung.RunPreCommit(t.Context(), deps(t, strict, "XDG_DATA_HOME="+home.Data)); out.Code != 1 || !contains(out, line) {
		t.Fatalf("strict = %+v, want the refusal of the strict set", out)
	}
}

//nolint:paralleltest // the fake gitleaks is found through PATH
func TestPreCommitSurfaceStepSwitchesOffAndSkipsOtherFiles(t *testing.T) {
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks})
	home := hometest.New(t)
	dir := installTool(t, home.Tools(), "ruff", ruffAt(t, home.Tools(), 3))
	off := armedRepo(t, armedPolicy+"ladder: {surface_lint: false}\n")
	off.Stage(map[string]string{staged: checkSource})
	if out := rung.RunPreCommit(t.Context(), deps(t, off, "XDG_DATA_HOME="+home.Data)); out.Code != 0 || !contains(out, "golaw pre-commit: surface: skipped, ladder.surface_lint is false") {
		t.Fatalf("switched off = %+v", out)
	}
	notes := armedRepo(t, armedPolicy)
	notes.Stage(map[string]string{"notes.md": "why\n", "views/panel.js": "run();\n"})
	if out := rung.RunPreCommit(t.Context(), deps(t, notes, "XDG_DATA_HOME="+home.Data)); out.Code != 0 || !contains(out, "golaw pre-commit: surface: skipped, no staged file a fast linter reads") {
		t.Fatalf("no file = %+v", out)
	}
	if calls := runnertest.Calls(t, dir, "ruff"); len(calls) != 0 {
		t.Fatalf("ruff calls = %q, want none", calls)
	}
}
