package cicheck_test

import (
	"strings"
	"testing"
)

const m6SetupPath = "docs/setup.md"

func m6SetupStatements() []string {
	return []string{
		"The languages of a repository other than Go are found from its files on every run; nothing needs to be enabled.",
		"golaw setup --surface downloads and verifies the tools the languages of the repository need",
		"it is the only command that reaches the network for them",
		"refuses a checksum mismatch naming the expected and the observed digest",
		"surface-tool-available ruff",
		"surface-tool-missing eslint",
		"surface-runtime-missing node",
		"A missing tool is a doctor line and a notice, never a denial",
		"golaw lint is the one command that treats a missing required tool as unmeasured and exits 2",
		"ladder.surface_lint: false removes the fast linters from the armed pre-commit rung",
		"ladder.hook_feedback: false stops the external tools and the non-certain feedback in the hook",
		"golaw init --surface-rules parity --apply",
		"A missing, empty, unparseable or stale report is unmeasured and exits 2.",
	}
}

func TestM6SetupGuidesTheSurface(t *testing.T) {
	t.Parallel()
	text := string(kitBytes(t, m6SetupPath))
	heading := "## The non-Go surface\n"
	commands := []string{
		"golaw setup --surface\n", "golaw doctor\n", "golaw lint --surface\n",
		"go test -coverprofile profile.cov ./...\n", "golaw coverage --new-code --input profile.cov --base origin/main\n",
	}
	statements := m6SetupStatements()
	if !m5InOrder(text, heading, commands) || !m4TextOK(text, statements, nil) {
		for _, statement := range statements {
			if !strings.Contains(text, statement) {
				t.Errorf("%s lacks %q", m6SetupPath, statement)
			}
		}
		t.Fatalf("%s does not carry the commands in order under %q", m6SetupPath, heading)
	}
	if m5InOrder(strings.Replace(text, commands[0], "golaw setup\n", 1), heading, commands) {
		t.Error("red: the section without the surface install passed")
	}
	if m4TextOK(strings.Replace(text, statements[7], "", 1), statements, nil) {
		t.Error("red: the section without the no-denial sentence passed")
	}
}
