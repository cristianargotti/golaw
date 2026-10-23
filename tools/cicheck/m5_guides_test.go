package cicheck_test

import (
	"strings"
	"testing"
)

const m5SetupPath = "docs/setup.md"

// m5InOrder reports whether every command appears after the heading, each
// after the one before it, the order a person runs them in.
func m5InOrder(text, heading string, commands []string) bool {
	_, rest, found := strings.Cut(text, heading)
	for _, command := range commands {
		if !found {
			return false
		}
		_, rest, found = strings.Cut(rest, command)
	}
	return found
}

func TestM5ProfileSetup(t *testing.T) {
	t.Parallel()
	text := string(kitBytes(t, m5SetupPath))
	commands := []string{
		"golaw profile install <git url>@<tag>\n",
		"golaw init --profile <name>\n",
		"golaw init --profile <name> --apply\n",
		"git add golaw.yml\n",
		"git commit -m \"feat: adopt the golaw profile\"\n",
		"golaw doctor\n",
	}
	required := []string{
		"A profile is optional and chosen by a person; core only is the default",
		"`Deixados para a plataforma:`",
		"`profile-installed` with its version, digest and the days since its date",
		"Every teammate installs the same bundle once with the command the pin names.",
		"never a silent core-only answer",
		"review a diff that changes only the version and the digest of the pin, and commit it, then",
		"install, refresh, rollback and remove never change a pin.",
		"golaw init --seed-debt --apply",
		"The debt set is seeded once",
		"never add or broaden one, which scan --staged and the armed pre-commit rung refuse.",
		"Run the printed command in a terminal: until the bundle loads, the agent guards deny every call of the agent, the install among them.",
		"Adopting a profile after the first init seeds nothing",
		"After a plain golaw init --apply, the refusal names how to start over before the first commit",
		"golaw profile install <source>@<version> with a credential that reads the bundle, before make ci",
		"then run golaw init --apply once more to enroll the committed pin",
		"A teammate who pulls a moved pin installs the new version with the command the pin names, then runs golaw init --apply to enroll the pulled pin.",
	}
	swapped := strings.Replace(text, commands[1], "golaw init --profile <name> --preview\n", 1)
	rows := []struct {
		name, text string
		want       bool
	}{
		{"repository", text, true},
		{"red-preview-missing", swapped, false},
		{"red-pin-claim-missing", strings.Replace(text, required[6], "", 1), false},
		{"red-debt-broadening-missing", strings.Replace(text, required[9], "", 1), false},
		{"red-ci-provisioning-missing", strings.Replace(text, required[13], "", 1), false},
		{"red-late-profile-missing", strings.Replace(text, required[11], "", 1), false},
		{"red-pin-enrollment-missing", strings.Replace(text, required[14], "", 1), false},
		{"red-teammate-step-missing", strings.Replace(text, required[15], "", 1), false},
		{"near-another-profile-name", text + "\ngolaw init --profile fixture\n", true},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got := m5InOrder(row.text, "## Adopt a profile\n", commands) && m4TextOK(row.text, required, nil)
			if got != row.want {
				t.Fatalf("profile setup = %v, want %v", got, row.want)
			}
		})
	}
}
