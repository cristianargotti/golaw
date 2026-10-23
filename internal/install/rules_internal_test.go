package install

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/checks/shell"
)

const cinRulesGolden = `# Generated from the hook's shared shell patterns.
# A prefix cannot express an option anywhere, git -C, or wrappers.
# Written only with --scope project; applies in this repository.
prefix_rule(pattern=["git","commit",["--no-verify","-n"]], decision="forbidden", justification="the command skips the git hooks; the hooks are the rule")
prefix_rule(pattern=["git",["merge","rebase","push"],"--no-verify"], decision="forbidden", justification="the command skips the git hooks; the hooks are the rule")
`

func TestCodexRulesGolden(t *testing.T) {
	t.Parallel()
	home := cinHome(t)
	got, err := CodexRules(shell.Prefixes())
	must(t, err)
	if string(got) != cinRulesGolden {
		t.Fatalf("rules = %q", got)
	}
	asset, err := assets.Read("agents/codex/golaw.rules")
	must(t, err)
	if !bytes.Equal(asset, got) {
		t.Fatal("embedded rules differ from the shared source")
	}
	path := filepath.Join(home, ".codex", "rules", "golaw.rules")
	var plan Plan
	must(t, addCodexRules(&plan, path, Receipt{}))
	if len(plan.Changes) != 1 || len(plan.Receipt.Artifacts) != 1 ||
		!bytes.Equal(plan.Changes[0].After, got) {
		t.Fatalf("rules plan = %+v", plan)
	}
	cinWrite(t, path, got)
	var repeated Plan
	must(t, addCodexRules(&repeated, path, plan.Receipt))
	if len(repeated.Changes) != 0 {
		t.Fatalf("repeated rules changes = %+v", repeated.Changes)
	}
}

func TestCodexRulesRejectInvalidPrefixes(t *testing.T) {
	t.Parallel()
	cinHome(t)
	for _, row := range []struct {
		name   string
		prefix shell.Prefix
	}{
		{"missing words", shell.Prefix{Check: "shell.no-verify", Reason: "reason"}},
		{"empty alternatives", shell.Prefix{
			Check: "shell.no-verify", Words: [][]string{{}}, Reason: "reason",
		}},
		{"empty word", shell.Prefix{
			Check: "shell.no-verify", Words: [][]string{{""}}, Reason: "reason",
		}},
		{"missing reason", shell.Prefix{
			Check: "shell.no-verify", Words: [][]string{{"git"}},
		}},
		{"optional policy", shell.Prefix{
			Check: "shell.push", Words: [][]string{{"git"}, {"push"}}, Reason: "reason",
		}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if _, err := CodexRules([]shell.Prefix{row.prefix}); !errors.Is(err, ErrSettings) {
				t.Fatalf("invalid prefix error = %v", err)
			}
		})
	}
}
