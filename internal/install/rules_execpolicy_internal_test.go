package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/shell"
)

func TestCodexRulesExecpolicy(t *testing.T) {
	t.Parallel()
	home := cinHome(t)
	binary, err := exec.LookPath("codex")
	if errors.Is(err, exec.ErrNotFound) {
		t.Skip("codex is absent from PATH; offline execpolicy validation was not run")
	}
	must(t, err)
	// Only the pinned CLI judges the rules: another build on a
	// contributor's PATH answers for itself, not for this kit.
	version, versionErr := exec.CommandContext(t.Context(), binary, "--version").Output()
	if got := strings.TrimSpace(string(version)); versionErr != nil || got != "codex-cli 0.163.0" {
		t.Skipf("the codex on PATH answers %q, not the pinned codex-cli 0.163.0; offline execpolicy validation was not run", got)
	}
	rules, err := CodexRules(shell.Prefixes())
	must(t, err)
	path := filepath.Join(home, ".codex", "rules", "golaw.rules")
	cinWrite(t, path, rules)
	for _, row := range []struct {
		name      string
		words     []string
		forbidden bool
	}{
		{"commit long", []string{"git", "commit", "--no-verify"}, true},
		{"commit short", []string{"git", "commit", "-n"}, true},
		{"merge", []string{"git", "merge", "--no-verify", "topic"}, true},
		{"rebase", []string{"git", "rebase", "--no-verify"}, true},
		{"push bypass", []string{"git", "push", "--no-verify"}, true},
		{"message value", []string{"git", "commit", "-m", "--no-verify"}, false},
		{"path operand", []string{"git", "commit", "--", "--no-verify"}, false},
		{"later option limit", []string{"git", "commit", "-m", "x", "-n"}, false},
		{"directory option limit", []string{"git", "-C", "repo", "commit", "-n"}, false},
		{"wrapper limit", []string{"bash", "-lc", "git commit -n"}, false},
		{"push default", []string{"git", "push", "origin", "main"}, false},
		{"request default", []string{"gh", "pr", "create"}, false},
		{"printer suffix", []string{"gh", "auth", "token", "--hostname", "example.test"}, false},
		{"printer empty continuation", []string{"op", "read"}, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			execpolicyRow(t, binary, home, path, row.words, row.forbidden)
		})
	}
}

// execpolicyRow runs one argv through codex execpolicy check and holds its
// answer to the row. Only stdout is decoded: when parallel runs race for
// the PATH aliases of one home, the pinned CLI prints a warning on stderr
// with exit 0 and the JSON intact, so stderr joins the failure line alone.
func execpolicyRow(t *testing.T, binary, home, path string, words []string, forbidden bool) {
	t.Helper()
	args := append([]string{"execpolicy", "check", "--rules", path}, words...)
	command := exec.CommandContext(t.Context(), binary, args...)
	command.Env = append(os.Environ(), cinEnvironment(home)...)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if runErr := command.Run(); runErr != nil {
		t.Fatalf("execpolicy: %v\n%s%s", runErr, stdout.Bytes(), stderr.Bytes())
	}
	output := stdout.Bytes()
	var answer struct {
		Decision     string            `json:"decision"`
		MatchedRules []json.RawMessage `json:"matchedRules"`
	}
	must(t, json.Unmarshal(output, &answer))
	if forbidden {
		if answer.Decision != "forbidden" || len(answer.MatchedRules) == 0 {
			t.Fatalf("missing prohibition: %s", output)
		}
	} else if answer.Decision != "" || len(answer.MatchedRules) != 0 {
		t.Fatalf("false prohibition: %s", output)
	}
}
