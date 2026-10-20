package rung_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/rung"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

// The pinned gitleaks a fake plays, its call over the staged diff, and the
// exit status it answers for a leak.
var (
	leaks      = runnertest.Behavior{Probe: []string{"version"}, Version: "8.30.1"}
	leaksCall  = []string{"git", "--pre-commit", "--staged", "--redact", "--no-banner", "--no-color", "--verbose", "--exit-code", "3", "."}
	notShipped = []string{
		"golaw pre-commit: migrate-lint: not applicable, this version ships no database template",
		"golaw pre-commit: openapi-breaking: not applicable, this version ships no api template",
		"golaw pre-commit: generate-check: not applicable, this version ships no generator template",
	}
)

// fakes puts fake tools first on PATH, git and go still found behind them,
// and answers their directory.
func fakes(t *testing.T, tools map[string]runnertest.Behavior) string {
	t.Helper()
	dir := t.TempDir()
	for name, b := range tools {
		runnertest.Install(t, dir, name, b)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

// answering is the fake gitleaks with the code and the output of a scan.
func answering(code int, stdout string) runnertest.Behavior {
	b := leaks
	b.Code, b.Stdout = code, stdout
	return b
}

//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitReportsTheStagedScanAndTheSecrets(t *testing.T) {
	dir := fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks})
	r := armedRepo(t, armedPolicy)
	r.Stage(map[string]string{"notes.md": "why\n"})
	clean := rung.RunPreCommit(t.Context(), deps(t, r))
	if clean.Code != 0 || !strings.HasPrefix(clean.Lines[0], "verdict clean: 0 findings") || clean.Lines[1] != "golaw pre-commit: secrets: gitleaks found no secret in the staged changes" ||
		!slices.Equal(clean.Lines[len(clean.Lines)-len(notShipped):], notShipped) {
		t.Errorf("clean commit = %+v", clean)
	}
	if calls := runnertest.Calls(t, dir, "gitleaks"); !slices.EqualFunc(calls, [][]string{{"version"}, leaksCall}, slices.Equal) {
		t.Errorf("gitleaks calls = %v", calls)
	}
	r.Stage(map[string]string{"docs/a.md": "one " + dash + " two\n"})
	dashed := rung.RunPreCommit(t.Context(), deps(t, r))
	human := "golaw bloqueou a operação: travessão no texto; use vírgula, dois pontos ou hífen (verificação text.dashes, arquivo docs/a.md, linha 1)."
	if dashed.Code != 1 || !contains(dashed, "docs/a.md:1:5 deny text.dashes") || dashed.Lines[len(dashed.Lines)-1] != human {
		t.Errorf("a staged dash = %+v", dashed)
	}
}

//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitRefusesWhatGitleaksFinds(t *testing.T) {
	r := armedRepo(t, armedPolicy)
	r.Stage(map[string]string{"notes.md": "why\n"})
	fakes(t, map[string]runnertest.Behavior{"gitleaks": answering(3, "Finding:     REDACTED\nFile:        notes.md\nLine:        1\n")})
	found := rung.RunPreCommit(t.Context(), deps(t, r))
	if found.Code != 1 || !contains(found, "golaw pre-commit: secrets: gitleaks found a secret in the staged changes; take it out of the commit") ||
		!contains(found, "File:        notes.md") || found.Lines[len(found.Lines)-1] != "golaw recusou o hook pre-commit; a causa e a correção estão nas linhas acima." {
		t.Errorf("a leak = %+v", found)
	}
	fakes(t, map[string]runnertest.Behavior{"gitleaks": answering(1, "")})
	if out := rung.RunPreCommit(t.Context(), deps(t, r)); out.Code != 2 || !contains(out, "gitleaks failed with exit status 1") {
		t.Errorf("a gitleaks failure = %+v", out)
	}
	other := leaks
	other.Version = "8.29.0"
	fakes(t, map[string]runnertest.Behavior{"gitleaks": other})
	if out := rung.RunPreCommit(t.Context(), deps(t, r)); out.Code != 2 || !contains(out, "pinned tool at another version") || !contains(out, "run golaw setup") {
		t.Errorf("gitleaks at another version = %+v", out)
	}
}

//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitSkipsTheSecretsOfNothingAndOfAMerge(t *testing.T) {
	dir := fakes(t, map[string]runnertest.Behavior{"gitleaks": answering(3, "")})
	r := armedRepo(t, armedPolicy)
	if out := rung.RunPreCommit(t.Context(), deps(t, r)); out.Code != 0 || !contains(out, "secrets: skipped, nothing staged") {
		t.Errorf("nothing staged = %+v", out)
	}
	r.Branch("side", map[string]string{"side.md": "side\n"})
	r.Commit("docs: say why", map[string]string{"notes.md": "why\n"})
	r.MergeInProgress("side")
	if out := rung.RunPreCommit(t.Context(), deps(t, r)); out.Code != 0 || !contains(out, "secrets: skipped, a merge is in progress") {
		t.Errorf("a merge in progress = %+v", out)
	}
	if calls := runnertest.Calls(t, dir, "gitleaks"); len(calls) != 0 {
		t.Errorf("gitleaks ran: %v", calls)
	}
}

//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitRefusesAWeakenedPolicyBeforeAnyTool(t *testing.T) {
	dir := fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks})
	r := armedRepo(t, armedPolicy+"checks: {sizes: {file_lines: 200}}\n")
	r.Stage(map[string]string{"golaw.yml": armedPolicy + "checks: {sizes: {file_lines: 300}}\n"})
	out := rung.RunPreCommit(t.Context(), deps(t, r))
	if out.Code != 2 || !strings.HasPrefix(out.Lines[0], "golaw pre-commit: staged scan: staged golaw.yml: ") || !contains(out, "git restore --staged golaw.yml") {
		t.Errorf("a weakened policy = %+v", out)
	}
	if calls := runnertest.Calls(t, dir, "gitleaks"); len(calls) != 0 {
		t.Errorf("gitleaks ran: %v", calls)
	}
	off := armedRepo(t, armedPolicy+"ladder: {pre_commit: false}\n")
	off.Stage(map[string]string{"docs/a.md": "one " + dash + " two\n"})
	if out := rung.RunPreCommit(t.Context(), deps(t, off)); out.Code != 0 || len(out.Lines) != 0 {
		t.Errorf("a rung switched off = %+v", out)
	}
}

//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitFailsFastOnALockAnotherCommitHolds(t *testing.T) {
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks})
	r := armedRepo(t, armedPolicy)
	r.Stage(map[string]string{"notes.md": "why\n"})
	lock, err := runner.Acquire(t.Context(), runner.LockPath(r.GitDir(), rung.PreCommit), runner.RungWait)
	if err != nil {
		t.Fatal(err)
	}
	out := rung.RunPreCommit(t.Context(), deps(t, r))
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
	if out.Code != 2 || !contains(out, "is held by process") || !contains(out, "wait for the commit that holds it to end, then commit again") {
		t.Errorf("a held lock = %+v", out)
	}
	if out := rung.RunPreCommit(t.Context(), deps(t, r)); out.Code != 0 {
		t.Errorf("after the release = %+v", out)
	}
}

// A staged file over the read bound is no finding the rung recognizes: it
// is named and left to the pipeline, and the commit goes on.
//
//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitLeavesToThePipelineAFileOverTheReadBound(t *testing.T) {
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks})
	r := armedRepo(t, armedPolicy)
	r.Stage(map[string]string{"dump.txt": strings.Repeat("a\n", 4<<20+1)})
	out := rung.RunPreCommit(t.Context(), deps(t, r))
	if out.Code != 0 || !contains(out, "golaw pre-commit: over the read bound, not judged here: dump.txt; the pipeline judges them") {
		t.Errorf("a staged file over the read bound = %+v", out)
	}
}

// A base over the read bound cannot disprove a finding the staged file
// keeps, so the rung names the file and lets the commit go, whether or not
// another staged file is over the bound too: the second file must not turn
// the doubt into a refusal.
//
//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitLeavesToThePipelineWhatAnUnreadBaseMayHold(t *testing.T) {
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks})
	big := strings.Repeat("a\n", 4<<20+1)
	kept := "old " + dash + " line\n"
	for named, staged := range map[string]map[string]string{
		"a.md":       {"a.md": kept},
		"a.md, b.md": {"a.md": kept, "b.md": big},
	} {
		r := armedRepo(t, armedPolicy)
		r.Commit("chore: add the dump", map[string]string{"a.md": kept + big})
		r.Stage(staged)
		out := rung.RunPreCommit(t.Context(), deps(t, r))
		if out.Code != 0 || !contains(out, "golaw pre-commit: over the read bound, not judged here: "+named+"; the pipeline judges them") || contains(out, "deny text.dashes") {
			t.Errorf("a shortened dump that keeps its old dash, staged with %s = %+v", named, out)
		}
	}
}

// The staged comparison masks the counts of a message and never the digits
// of a name, so a new long function named like an old one but for a digit
// is refused as one named apart is.
//
//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitRefusesANewFindingNamedLikeAnOldOne(t *testing.T) {
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks, "golangci-lint": linter})
	long := func(name string) string {
		return "// " + name + " counts.\nfunc " + name + "() int {\n\tx := 0\n" + strings.Repeat("\tx++\n", 45) + "\treturn x\n}\n"
	}
	for _, added := range []string{"Step2", "StepB"} {
		r := armedRepo(t, armedPolicy+"capabilities: [go]\n")
		r.Commit("feat: add the step", map[string]string{"a/a.go": "package a\n\n" + long("Step1")})
		r.Stage(map[string]string{"a/a.go": "package a\n\n// Step1 counts.\nfunc Step1() int { return 1 }\n\n" + long(added)})
		out := rung.RunPreCommit(t.Context(), deps(t, r))
		if out.Code != 1 || !contains(out, "deny goast.function-length function "+added+" has 47 statements") {
			t.Errorf("Step1 shortened and a new long function %s = %+v", added, out)
		}
	}
}
