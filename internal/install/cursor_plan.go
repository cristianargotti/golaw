package install

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const (
	cursorAgent     = "cursor"
	cursorLocalDir  = ".cursor"
	cursorHooksName = "hooks.json"
	// The folders below the home or the repository root a destination
	// passes through, the file itself included: .cursor/hooks.json,
	// .cursor/rules/golaw.mdc and .agents/skills/golaw/SKILL.md.
	cursorHooksDepth = 2
	cursorRuleDepth  = 3
	cursorSkillDepth = 4
)

// cursorPreviewNote is the line every Cursor install ends with: Cursor is
// a compatibility preview until a live record proves its Write payload.
const cursorPreviewNote = "O Cursor é uma prévia de compatibilidade: as escritas dele são capturadas, mas ainda não bloqueadas (veja docs/agent-contracts.md)"

// cursorPlan resolves where a Cursor installation writes and reads the
// receipt an earlier one left: the receipt layout is the one every agent
// shares, built here because the agent registry imports this package.
func cursorPlan(options Options) (Plan, Receipt, error) {
	scope, err := cursorScope(options)
	if err != nil {
		return Plan{}, Receipt{}, err
	}
	root, identity, err := ProjectIdentity(options.Root)
	if err != nil {
		return Plan{}, Receipt{}, err
	}
	directory, store := options.ConfigDir, filepath.Join(options.StateDir, cursorAgent, scope)
	if scope == scopeProject {
		directory, store = filepath.Join(root, cursorLocalDir), filepath.Join(store, identity)
	}
	receiptPath := filepath.Join(store, receiptName)
	previous, err := previousCursorReceipt(receiptPath, scope)
	if err != nil {
		return Plan{}, Receipt{}, err
	}
	receipt := Receipt{
		Schema: 1, ProjectID: identity, Scope: scope, Version: options.Version,
		Bundle: store, Agent: cursorAgent, HooksPath: filepath.Join(directory, cursorHooksName),
	}
	return Plan{
		Receipt: receipt, ReceiptPath: receiptPath,
		PolicyDigest: options.PolicyDigest, ProfileDigest: options.ProfileDigest,
	}, previous, nil
}

// cursorScope answers the scope of the installation, user when none is
// given, after checking that every directory the plan joins is absolute:
// a relative one would write wherever the process happens to run.
func cursorScope(options Options) (string, error) {
	scope := options.Scope
	if scope == "" {
		scope = scopeUser
	}
	if scope != scopeUser && scope != scopeProject {
		return "", fmt.Errorf("cursor scope %q is neither user nor project: %w", scope, ErrPath)
	}
	names := []string{"home", "state directory", "configuration directory", "running binary"}
	for index, path := range []string{options.Home, options.StateDir, options.ConfigDir, options.Executable} {
		if !filepath.IsAbs(path) {
			return "", fmt.Errorf("cursor %s %q is not an absolute path: %w", names[index], path, ErrPath)
		}
	}
	return scope, nil
}

// previousCursorReceipt reads the receipt an earlier installation of the
// scope wrote, none when it is absent; a receipt of another agent or scope
// at this path is evidence golaw did not write there and is refused.
func previousCursorReceipt(path, scope string) (Receipt, error) {
	previous, err := ReadReceipt(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Receipt{}, nil
	}
	if err != nil {
		return Receipt{}, err
	}
	if previous.Agent != cursorAgent || previous.Scope != scope {
		return Receipt{}, fmt.Errorf("receipt %s names agent %q at scope %q: %w", path, previous.Agent, previous.Scope, ErrPath)
	}
	return previous, nil
}

// cursorReceipt schedules the receipt after every other change, so a run
// that stops early never leaves a receipt naming files it did not write.
func cursorReceipt(plan *Plan) error {
	encoded, err := json.MarshalIndent(plan.Receipt, "", "  ")
	if err != nil {
		return fmt.Errorf("encode the cursor receipt: %w", err)
	}
	return addChange(plan, plan.ReceiptPath, append(encoded, '\n'), false)
}

// cursorUnlinked refuses a destination whose last depth components hold a
// symbolic link: a .cursor or .agents folder a repository ships as a link
// would make the install write, or the uninstall remove, wherever the link
// points.
func cursorUnlinked(path string, depth int) error {
	current := path
	for range depth {
		info, err := os.Lstat(current)
		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			return fmt.Errorf("inspect %s: %w", current, err)
		case info.Mode()&fs.ModeSymlink != 0:
			return fmt.Errorf("%s is a symbolic link: golaw changes the Cursor files only through real folders, so replace the link with the folder it points to: %w",
				current, ErrPath)
		}
		current = filepath.Dir(current)
	}
	return nil
}
