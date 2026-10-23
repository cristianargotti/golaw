package cicheck_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook/agents"
)

func TestM4SetupCommandsAndPaths(t *testing.T) {
	t.Parallel()
	const path = "docs/setup.md"
	ids := agents.IDs()
	if len(ids) != 3 {
		t.Fatal("expected three registered agents")
	}
	for _, id := range []string{ids[1], ids[2], "all"} {
		for _, action := range []string{"install", "uninstall"} {
			for _, scope := range []string{"", " --scope project"} {
				requireText(t, path, "golaw agent "+action+" --agent "+id+scope)
			}
		}
	}
	for _, fragment := range []string{
		"--scope user explicitly selects the default scope.",
		"~/." + ids[1] + "/hooks.json",
		"~/." + ids[1] + "/AGENTS.md",
		"." + ids[1] + "/hooks.json",
		"." + ids[1] + "/rules/golaw.rules",
		"~/." + ids[2] + "/hooks.json",
		"." + ids[2] + "/hooks.json",
		"." + ids[2] + "/rules/golaw.mdc",
		"~/.agents/skills/golaw/SKILL.md",
		".agents/skills/golaw/SKILL.md",
		strings.ToUpper(ids[1]) + "_HOME",
		"/hooks",
		"hooks.state",
	} {
		requireText(t, path, fragment)
	}
}

func TestM4SetupAssuranceAndRemoval(t *testing.T) {
	t.Parallel()
	text := string(kitBytes(t, "docs/setup.md"))
	ids := agents.IDs()
	if len(ids) != 3 {
		t.Fatal("expected three registered agents")
	}
	required := []string{
		"Installed, trusted and active are separate facts.",
		"protecting requires matching installation, established trust and current observed dispatch.",
		"doctor exits 0 after printing a report, including findings; collection failure exits 2.",
		"doctor --json prints one JSON document per agent.",
		"Project trust and hook trust are separate steps.",
		"The installer never writes hooks.state.",
		"No user-scope rule file is installed for the compatibility preview.",
		m4AgentName(ids[1]) + " prefix rules are written only with --scope project; a user-scope rules file would apply to every " + m4AgentName(ids[1]) + " session, enrolled or not.",
		"Uninstall keeps a shared skill while another receipt owns it.",
		"A changed owned file is refused with ErrChanged and is not removed.",
		"Capture does not turn on write enforcement.",
		m4AgentName(ids[1]) + " uses features.hooks in config.toml, on by default; enable it if it is disabled.",
		m4AgentName(ids[1]) + " stops reading the AGENTS.md chain at 32 KiB and the golaw block stays under 1,000 bytes; doctor.agents-chain measures the AGENTS.md at the repository root against that budget.",
		"a root file over the budget is certain for every session",
		m4AgentName(ids[2]) + " imports " + m4AgentName(ids[0]) + " settings-file hooks by default and maps Edit to Write and Bash to Shell, so a golaw hook command in ~/." + ids[0] + "/settings.json would run twice; the hooks live in the plugin, and doctor reports duplicate-hook.",
		"A golaw skill under ~/." + ids[1] + "/skills or ~/." + ids[2] + "/skills beside ~/.agents/skills produces skill-duplicate.",
		"duplicate-hook", "skill-duplicate",
		"doctor.installed",
		"doctor.trusted",
		"doctor.dispatch",
		"doctor.state",
		"doctor.fail-mode",
		"doctor.version",
		"doctor.preview",
		"doctor.capture",
		"doctor.duplicate",
		"doctor.companion",
		"doctor.agents-chain",
		"agents-chain-over",
		"host-fails-open",
		"revalidation-required",
		"trusted-inactive",
		"preview-active",
		"An agent with no receipt at a scope is not-installed in state unknown, never red.",
		m4AgentName(ids[2]) + " is never protecting: an installed, trusted and observed " + m4AgentName(ids[2]) + " hook is preview-active, because its writes are captured, not enforced.",
	}
	forbidden := []string{
		"A zero doctor exit code proves protection.",
		"Installing an agent arms the git rungs.",
		"doctor does not measure the chain in M4.",
	}
	rows := []struct {
		name, text string
		want       bool
	}{
		{"repository", text, true},
		{"red-missing-trust", strings.ReplaceAll(text, required[0], ""), false},
		{"red-exit-overclaim", text + "\n" + forbidden[0], false},
		{"red-rung-overclaim", text + "\n" + forbidden[1], false},
		{"red-unmeasured-chain", text + "\n" + forbidden[2], false},
		{"near-other-installation", text + "\nForeign entries retain their order.\n", true},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if got := m4TextOK(row.text, required, forbidden); got != row.want {
				t.Fatalf("setup contract = %v, want %v", got, row.want)
			}
		})
	}
}
