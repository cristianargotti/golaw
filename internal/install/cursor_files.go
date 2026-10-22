package install

import (
	"fmt"
	"path/filepath"

	"github.com/cristianargotti/golaw/internal/assets"
)

const (
	cursorRuleAsset = "agents/cursor/golaw.mdc"
	cursorRulesDir  = "rules"
	cursorRuleName  = "golaw.mdc"
)

// cursorFiles plans the files a Cursor installation owns whole: the project
// rule with --scope project only, because Cursor documents no user rules
// file, and the shared skill both hooks file agents read.
func cursorFiles(plan *Plan, options Options, previous Receipt) error {
	if plan.Receipt.Scope == scopeProject {
		if err := cursorRule(plan, previous); err != nil {
			return err
		}
	}
	if err := cursorUnlinked(SkillPath(options), cursorSkillDepth); err != nil {
		return err
	}
	return addSharedSkill(plan, options)
}

// cursorRulePath is derived from the receipt, so uninstall finds the rule
// without the options of the install that wrote it.
func cursorRulePath(receipt Receipt) string {
	return filepath.Join(filepath.Dir(receipt.HooksPath), cursorRulesDir, cursorRuleName)
}

// cursorRule plans the project rule from its asset, owned whole: a rule the
// person or another tool changed is refused, never overwritten.
func cursorRule(plan *Plan, previous Receipt) error {
	path := cursorRulePath(plan.Receipt)
	if err := cursorUnlinked(path, cursorRuleDepth); err != nil {
		return err
	}
	source, err := assets.Read(cursorRuleAsset)
	if err != nil {
		return fmt.Errorf("load the cursor rule: %w", err)
	}
	if err := checkAssetOwner(path, previous); err != nil {
		return fmt.Errorf("the cursor rule %s is not the one golaw installed: restore it or move it away: %w", path, err)
	}
	return addArtifact(plan, path, source)
}
