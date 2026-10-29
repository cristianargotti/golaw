package cicheck_test

import (
	"strings"
	"testing"
)

const m6SecurityPath = "docs/security.md"

func m6SecurityStatements() []string {
	return []string{
		"Three tools are the one exception, on the post event of a write only: ruff, hadolint and shellcheck",
		"only from the kit's tool directory with a receipt that matches",
		"with the kit's own configuration and the repository's configuration lookup disabled",
		"an environment reduced to PATH, HOME, TMPDIR and the locale",
		"None of the three executes code it reads.",
		"ESLint, stylelint, pyright, cfn-lint and checkov never run in a hook",
		"## Surface tools",
		"golaw setup --surface is the only command that downloads a surface tool",
		"No hook reaches the network.",
		"with their install scripts disabled",
		"shellcheck and shfmt publish no checksum, so their pin is trust on first use",
		"Node and Python are prerequisites the kit never installs",
		"run outside their declared ESLint peer range",
		"A missing tool, a missing runtime or a checksum drift is never a denial and never a finding in a hook or a rung.",
		"golaw never calls a Sonar server, a scanner or its API and never reads a token for it.",
	}
}

func TestM6SecurityStatesTheSurfaceBoundary(t *testing.T) {
	t.Parallel()
	text := string(kitBytes(t, m6SecurityPath))
	statements := m6SecurityStatements()
	forbidden := []string{"ESLint runs in the hook", "the hook downloads", "--isolated"}
	if !m4TextOK(text, statements, forbidden) {
		for _, statement := range statements {
			if !strings.Contains(text, statement) {
				t.Errorf("%s lacks %q", m6SecurityPath, statement)
			}
		}
		t.FailNow()
	}
	requireText(t, m6SecurityPath, "execute no repository code")
	for _, red := range []string{statements[4], statements[8], statements[10]} {
		if m4TextOK(strings.Replace(text, red, "", 1), statements, forbidden) {
			t.Errorf("red: the page without %q passed", red)
		}
	}
}
