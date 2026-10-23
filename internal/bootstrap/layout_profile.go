package bootstrap

import "github.com/cristianargotti/golaw/internal/model"

// ProfileLayout is the layout a new policy renders under a profile (D14):
// a field the repository shows nothing for takes the default of the
// profile (the main layout without main.go at the root and without a cmd
// directory, the decisions directory without docs/adr and without
// docs/decisions), and a field the repository shows keeps what Detect
// found, so a default never contradicts the tree.
func ProfileLayout(root string, detected Layout, defaults *model.Layout) Layout {
	if defaults == nil {
		return detected
	}
	if defaults.Main != "" && !exists(root, rootMain) && !exists(root, cmdDir) {
		detected.Main = defaults.Main
	}
	if defaults.Decisions != "" && !exists(root, defaultDecisions) && !exists(root, legacyDecisions) {
		detected.Decisions = defaults.Decisions
	}
	return detected
}
