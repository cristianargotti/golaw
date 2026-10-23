//go:build releaseprobe

package cicheck_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const releaseGatePrelude = `
git() {
  case "$1" in
    rev-parse) printf '%s\n' "$CASE_SHA" ;;
    merge-base) test "$CASE_ANCESTOR" = yes ;;
    *) return 9 ;;
  esac
}
gh() {
  test "$CASE_API" = yes || return 8
  printf '%s\n' "$CASE_JSON"
}
go() {
  test "$1 $2" = "run ./tools/releasegate" || return 7
  shift 2
  "$CASE_GATE" "$@"
}
source tools/cicheck/release-gate.sh
`

type releaseGateCase struct {
	name     string
	refType  string
	ancestor string
	api      string
	runs     []map[string]any
	pass     bool
}

func releaseGateCases(sha string) []releaseGateCase {
	run := func(head, branch, event, status, conclusion string, number int) map[string]any {
		return map[string]any{
			"head_sha": head, "head_branch": branch, "event": event,
			"status": status, "conclusion": conclusion,
			"run_number": number, "run_attempt": 1,
		}
	}
	green := run(sha, "main", "push", "completed", "success", 1)
	return []releaseGateCase{
		{"green", "tag", "yes", "yes", []map[string]any{green}, true},
		{"empty", "tag", "yes", "yes", nil, false},
		{"branch", "branch", "yes", "yes", []map[string]any{green}, false},
		{"not ancestor", "tag", "no", "yes", []map[string]any{green}, false},
		{"API failure", "tag", "yes", "no", []map[string]any{green}, false},
		{"wrong SHA", "tag", "yes", "yes", []map[string]any{
			run(strings.Repeat("b", 40), "main", "push", "completed", "success", 1),
		}, false},
		{"wrong branch", "tag", "yes", "yes", []map[string]any{
			run(sha, "topic", "push", "completed", "success", 1),
		}, false},
		{"wrong event", "tag", "yes", "yes", []map[string]any{
			run(sha, "main", "pull_request", "completed", "success", 1),
		}, false},
		{"new failure", "tag", "yes", "yes", []map[string]any{
			green, run(sha, "main", "push", "completed", "failure", 2),
		}, false},
		{"new pending", "tag", "yes", "yes", []map[string]any{
			green, run(sha, "main", "push", "in_progress", "", 2),
		}, false},
	}
}

func requireReleaseGateRecord(t *testing.T, path, sha string, pass bool) {
	t.Helper()
	record, err := os.ReadFile(path)
	if !pass {
		if len(record) != 0 {
			t.Fatalf("rejected tag received an output: %s", record)
		}
		return
	}
	if err != nil || string(record) != "sha="+sha+"\n" {
		t.Fatalf("wrong eligibility output: %q, %v", record, err)
	}
}

// releaseGateTool builds the gate tool once, so the script's go run line
// runs the decision the lane runs without the toolchain in the probe's
// environment.
func releaseGateTool(t *testing.T) string {
	t.Helper()
	tool := filepath.Join(t.TempDir(), "releasegate")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", tool, "./tools/releasegate")
	build.Dir = filepath.Join("..", "..")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build the gate tool: %v\n%s", err, output)
	}
	return tool
}

func TestReleaseGateDecisions(t *testing.T) {
	t.Parallel()
	sha := strings.Repeat("a", 40)
	tool := releaseGateTool(t)
	for _, test := range releaseGateCases(sha) {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			payload, err := json.Marshal(map[string]any{"workflow_runs": test.runs})
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			outputPath := filepath.Join(dir, "output")
			command := exec.CommandContext(t.Context(), "bash", "-c", releaseGatePrelude)
			command.Dir = filepath.Join("..", "..")
			command.Env = []string{
				"PATH=" + os.Getenv("PATH"),
				"CASE_SHA=" + sha, "CASE_ANCESTOR=" + test.ancestor,
				"CASE_API=" + test.api, "CASE_JSON=" + string(payload), "CASE_GATE=" + tool,
				"GITHUB_REF_TYPE=" + test.refType,
				"GITHUB_REF=refs/tags/v0.1.0",
				"GITHUB_REPOSITORY=cristianargotti/golaw",
				"RUNNER_TEMP=" + dir, "GITHUB_OUTPUT=" + outputPath,
			}
			output, runErr := command.CombinedOutput()
			if (runErr == nil) != test.pass {
				t.Fatalf("pass=%v, error=%v, output=%s", test.pass, runErr, output)
			}
			requireReleaseGateRecord(t, outputPath, sha, test.pass)
		})
	}
}
