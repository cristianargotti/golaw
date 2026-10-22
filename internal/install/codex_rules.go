package install

import "github.com/cristianargotti/golaw/internal/checks/shell"

// addCodexRules plans the rules file of a Codex installation at path from
// the shared prefix table, owned whole as a bundle asset is: a file the
// person or another tool changed is refused with ErrChanged, never
// overwritten, and the file is a regular one, because the host never
// loads a rules file through a link.
func addCodexRules(plan *Plan, path string, previous Receipt) error {
	rules, err := CodexRules(shell.Prefixes())
	if err != nil {
		return err
	}
	if err := checkAssetOwner(path, previous); err != nil {
		return err
	}
	return addArtifact(plan, path, rules)
}
