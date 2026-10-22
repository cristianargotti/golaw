package install

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
)

const (
	codexAgent     = "codex"
	codexHooksName = "hooks.json"
	codexLocalDir  = ".codex"
)

// prepareCodex resolves where a Codex installation writes and reads the
// receipt an earlier one left: the receipt layout is the one every agent
// shares, built here because the agent registry imports this package.
func prepareCodex(options Options) (Plan, Receipt, error) {
	scope, err := codexScope(options)
	if err != nil {
		return Plan{}, Receipt{}, err
	}
	root, identity, err := ProjectIdentity(options.Root)
	if err != nil {
		return Plan{}, Receipt{}, err
	}
	directory, err := codexDirectory(options, scope, root)
	if err != nil {
		return Plan{}, Receipt{}, err
	}
	store := filepath.Join(options.StateDir, codexAgent, scope)
	if scope == scopeProject {
		store = filepath.Join(store, identity)
	}
	receiptPath := filepath.Join(store, receiptName)
	previous, err := previousCodexReceipt(receiptPath, scope)
	if err != nil {
		return Plan{}, Receipt{}, err
	}
	receipt := Receipt{
		Schema: 1, ProjectID: identity, Scope: scope, Version: options.Version,
		Bundle: store, Agent: codexAgent, HooksPath: filepath.Join(directory, codexHooksName),
	}
	return Plan{
		Receipt: receipt, ReceiptPath: receiptPath,
		PolicyDigest: options.PolicyDigest, ProfileDigest: options.ProfileDigest,
	}, previous, nil
}

// codexScope answers the scope of the installation, user when none is
// given, after checking that every directory the plan joins is absolute:
// a relative one would write wherever the process happens to run.
func codexScope(options Options) (string, error) {
	scope := options.Scope
	if scope == "" {
		scope = scopeUser
	}
	if scope != scopeUser && scope != scopeProject {
		return "", fmt.Errorf("codex scope %q: %w", scope, ErrPath)
	}
	for _, input := range []struct{ name, path string }{
		{"home", options.Home},
		{"state directory", options.StateDir},
		{"configuration directory", options.ConfigDir},
		{"running binary", options.Executable},
	} {
		if !filepath.IsAbs(input.path) {
			return "", fmt.Errorf("codex %s %q is not an absolute path: %w", input.name, input.path, ErrPath)
		}
	}
	return scope, nil
}

// codexDirectory is the folder whose hooks file Codex reads for the scope,
// named the way Codex keys hook trust by it: a CODEX_HOME the person set is
// resolved through every link, as Codex resolves it, while the default home
// is named as given, as Codex names it, so a handler the person approved is
// found under the same key.
func codexDirectory(options Options, scope, root string) (string, error) {
	if scope == scopeProject {
		return filepath.Join(root, codexLocalDir), nil
	}
	if !options.ConfigDirSet {
		return options.ConfigDir, nil
	}
	resolved, err := filepath.EvalSymlinks(options.ConfigDir)
	if errors.Is(err, fs.ErrNotExist) {
		return options.ConfigDir, nil
	}
	if err != nil {
		return "", fmt.Errorf("resolve the codex home %s: %w", options.ConfigDir, err)
	}
	return resolved, nil
}

// previousCodexReceipt reads the receipt an earlier installation of the
// scope wrote, none when it is absent; a receipt of another agent or scope
// at this path is corrupted evidence and is refused instead of reused.
func previousCodexReceipt(path, scope string) (Receipt, error) {
	previous, err := ReadReceipt(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Receipt{}, nil
	}
	if err != nil {
		return Receipt{}, err
	}
	if previous.Agent != codexAgent || previous.Scope != scope {
		return Receipt{}, fmt.Errorf("receipt %s names agent %q at scope %q: %w", path, previous.Agent, previous.Scope, ErrSettings)
	}
	return previous, nil
}
