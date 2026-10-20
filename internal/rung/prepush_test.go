package rung_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/rung"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

// Two findings govulncheck reports reachable: one of the standard library
// a waiver may hold, and one of a dependency without a fix.
const (
	serveFinding = `{"finding": {"osv": "GO-2026-0002", "fixed_version": "v1.27.2", "trace": [{"module": "stdlib", "package": "net/http", "function": "Serve"}]}}`
	depFinding   = `{"finding": {"osv": "GO-2026-0003", "trace": [{"module": "example.test/dep", "package": "example.test/dep", "function": "Do"}]}}`
)

// scanned is the fake govulncheck answering a stream of the scanner
// version with the findings.
func scanned(version string, findings ...string) runnertest.Behavior {
	config := `{"config": {"scanner_name": "govulncheck", "scanner_version": "` + version + `"}}`
	return runnertest.Behavior{Stdout: strings.Join(append([]string{config}, findings...), "\n") + "\n"}
}

// pushing are the dependencies of a push of the ref lines.
func pushing(t *testing.T, r *gittest.Repo, lines ...string) rung.Dependencies {
	t.Helper()
	d := deps(t, r)
	d.Stdin = strings.NewReader(strings.Join(lines, "\n"))
	return d
}

// update is the ref line of a push of a tip to a new ref.
func update(tip string) string {
	return "refs/heads/main " + tip + " refs/heads/main " + runner.ZeroSHA
}

//nolint:paralleltest // the fake tools are found through PATH
func TestPrePushJudgesTheReachableAdvisoriesByTheWaivers(t *testing.T) {
	waivers := "vulnerability_waivers:\n  - {id: GO-2026-0002, until: 2026-12-31, record: docs/adr/0003-serve.md}\n"
	r := armedRepo(t, armedPolicy+waivers)
	cases := map[string]struct {
		scan rung.Outcome
		b    runnertest.Behavior
	}{
		"nothing reachable": {rung.Outcome{Code: 0, Lines: []string{"govulncheck finds no reachable vulnerability"}}, scanned("v1.7.0")},
		"a waived advisory": {rung.Outcome{Code: 0, Lines: []string{"GO-2026-0002 waived until 2026-12-31 (docs/adr/0003-serve.md)"}}, scanned("v1.7.0", serveFinding)},
		"an unwaived one":   {rung.Outcome{Code: 1, Lines: []string{"GO-2026-0003 reachable in example.test/dep, fixed in no version yet"}}, scanned("v1.7.0", serveFinding, depFinding)},
	}
	for name, c := range cases {
		dir := fakes(t, map[string]runnertest.Behavior{"govulncheck": c.b, "gitleaks": leaks, "golangci-lint": linter})
		out := rung.RunPrePush(t.Context(), pushing(t, r, update(r.Head())), "origin", "https://example.com/repo.git")
		if out.Code != c.scan.Code || !contains(out, "golaw pre-push: "+r.Head()[:12]+": "+c.scan.Lines[0]) {
			t.Errorf("%s = %+v", name, out)
		}
		if calls := runnertest.Calls(t, dir, "govulncheck"); len(calls) != 1 || strings.Join(calls[0], " ") != "-format json ./..." {
			t.Errorf("%s: govulncheck calls = %v", name, calls)
		}
		if len(runnertest.Calls(t, dir, "gitleaks"))+len(runnertest.Calls(t, dir, "golangci-lint")) != 0 {
			t.Errorf("%s: the push rung ran a tool of the commit", name)
		}
	}
	expired := armedRepo(t, armedPolicy+"vulnerability_waivers:\n  - {id: GO-2026-0002, until: 2026-10-01, record: docs/adr/0003-serve.md}\n")
	fakes(t, map[string]runnertest.Behavior{"govulncheck": scanned("v1.7.0", serveFinding)})
	out := rung.RunPrePush(t.Context(), pushing(t, expired, update(expired.Head())), "origin", "")
	if out.Code != 1 || !contains(out, "GO-2026-0002 reachable in stdlib, fixed in v1.27.2; its waiver expired on 2026-10-01 (docs/adr/0003-serve.md)") ||
		out.Lines[len(out.Lines)-1] != "golaw recusou o hook pre-push; a causa e a correção estão nas linhas acima." {
		t.Errorf("an expired waiver = %+v", out)
	}
}

//nolint:paralleltest // the fake tools are found through PATH
func TestPrePushLetsAScanWithoutVerdictGo(t *testing.T) {
	r := armedRepo(t, armedPolicy)
	for name, b := range map[string]runnertest.Behavior{
		"an unreachable database":  {Stderr: "govulncheck: fetching vulnerabilities: dial tcp: lookup vuln.go.dev: no such host\n", Code: 1},
		"a stream that is no scan": {Stdout: `{"progress": {"message": "Scanning"}}` + "\n"},
	} {
		fakes(t, map[string]runnertest.Behavior{"govulncheck": b})
		out := rung.RunPrePush(t.Context(), pushing(t, r, update(r.Head())), "origin", "")
		if out.Code != 0 || len(out.Lines) != 1 || !contains(out, "govulncheck could not judge it, the pipeline judges it") {
			t.Errorf("%s = %+v", name, out)
		}
	}
	fakes(t, map[string]runnertest.Behavior{"govulncheck": scanned("v1.6.0")})
	out := rung.RunPrePush(t.Context(), pushing(t, r, update(r.Head())), "origin", "")
	if out.Code != 2 || !contains(out, "pinned tool at another version: govulncheck answers v1.6.0 and the pin is 1.7.0: run golaw setup") {
		t.Errorf("another scanner = %+v", out)
	}
	if out := rung.RunPrePush(t.Context(), pushing(t, r, "refs/heads/main"), "origin", ""); out.Code != 0 || !contains(out, "the ref lines git gave") {
		t.Errorf("a ref line git never writes = %+v", out)
	}
}

//nolint:paralleltest // the fake tools are found through PATH
func TestPrePushScansEveryPushedTipOnce(t *testing.T) {
	dir := fakes(t, map[string]runnertest.Behavior{"govulncheck": scanned("v1.7.0")})
	r := armedRepo(t, armedPolicy)
	side := r.Branch("side", map[string]string{"side.md": "side\n"})
	d := pushing(t, r, update(r.Head()), "refs/heads/release "+r.Head()+" refs/heads/release "+runner.ZeroSHA, "(delete) "+runner.ZeroSHA+" refs/heads/old "+side)
	if out := rung.RunPrePush(t.Context(), d, "origin", ""); out.Code != 0 || len(out.Lines) != 1 {
		t.Errorf("two refs of one tip and a deletion = %+v", out)
	}
	if calls := runnertest.Calls(t, dir, "govulncheck"); len(calls) != 1 {
		t.Errorf("govulncheck ran %d times", len(calls))
	}
	if _, err := os.Stat(filepath.Join(d.State, "runner")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("HEAD of a clean worktree is read in place: %v", err)
	}
	copied := pushing(t, r, "refs/heads/side "+side+" refs/heads/side "+runner.ZeroSHA)
	if out := rung.RunPrePush(t.Context(), copied, "origin", ""); out.Code != 0 || !emptyDir(t, filepath.Join(copied.State, "runner", "trees")) {
		t.Errorf("a tip that is not HEAD = %+v", out)
	}
	r.Write("notes.md", "dirty\n")
	dirty := pushing(t, r, update(r.Head()))
	if out := rung.RunPrePush(t.Context(), dirty, "origin", ""); out.Code != 0 || !emptyDir(t, filepath.Join(dirty.State, "runner", "trees")) {
		t.Errorf("HEAD of a worktree with an untracked file = %+v", out)
	}
	if out := rung.RunPrePush(t.Context(), pushing(t, r), "origin", ""); out.Code != 0 || len(out.Lines) != 0 {
		t.Errorf("a push of nothing = %+v", out)
	}
}

//nolint:paralleltest // the fake tools are found through PATH
func TestPrePushSwitchedOffReadsNoInput(t *testing.T) {
	dir := fakes(t, map[string]runnertest.Behavior{"govulncheck": scanned("v1.7.0", depFinding)})
	r := armedRepo(t, "golaw: 1\nkit: {version: 0.0.9}\nladder: {pre_push: false}\n")
	if out := rung.RunPrePush(t.Context(), pushing(t, r, "malformed"), "origin", ""); out.Code != 0 || len(out.Lines) != 0 {
		t.Errorf("a rung switched off = %+v", out)
	}
	if calls := runnertest.Calls(t, dir, "govulncheck"); len(calls) != 0 {
		t.Errorf("govulncheck ran: %v", calls)
	}
}

// An ignored Go file is no part of the pushed tip, yet govulncheck reads it
// in place, so a push of HEAD beside one scans a copy of the tip; an
// ignored file go never reads keeps the worktree read in place.
//
//nolint:paralleltest // the fake tools are found through PATH
func TestPrePushScansACopyBesideAGoFileGitIgnores(t *testing.T) {
	fakes(t, map[string]runnertest.Behavior{"govulncheck": scanned("v1.7.0")})
	r := armedRepo(t, armedPolicy)
	r.Commit("chore: ignore the local files", map[string]string{".gitignore": "local.go\n*.log\n"})
	r.Write("run.log", "ran\n")
	inPlace := pushing(t, r, update(r.Head()))
	if out := rung.RunPrePush(t.Context(), inPlace, "origin", ""); out.Code != 0 || len(out.Lines) != 1 {
		t.Errorf("HEAD beside an ignored log = %+v", out)
	}
	if _, err := os.Stat(filepath.Join(inPlace.State, "runner")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("an ignored file go never reads keeps HEAD read in place: %v", err)
	}
	r.Write("local.go", "package m\n\nimport _ \"example.test/vulnerable\"\n")
	copied := pushing(t, r, update(r.Head()))
	if out := rung.RunPrePush(t.Context(), copied, "origin", ""); out.Code != 0 || len(out.Lines) != 1 || !emptyDir(t, filepath.Join(copied.State, "runner", "trees")) {
		t.Errorf("HEAD beside an ignored Go file = %+v", out)
	}
}
