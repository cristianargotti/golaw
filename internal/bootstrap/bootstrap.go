// Package bootstrap prepares reviewed policy and template changes before init writes them.
// It never arms a git hook or reads the clock.
package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
)

// PolicyFile is the name of the policy file at the repository root.
const PolicyFile = "golaw.yml"

// The files and directories init looks at before it renders.
const (
	gitEntry         = ".git"
	goModule         = "go.mod"
	rootMain         = "main.go"
	cmdDir           = "cmd"
	defaultDecisions = "docs/adr"
	legacyDecisions  = "docs/decisions"
)

// The layouts of the main package (specification section 5.3).
const (
	MainRoot = "root"
	MainCmd  = "cmd"
)

// Layout is what init detects in the repository before rendering.
type Layout struct {
	Main      string // MainRoot when main.go sits at the root, MainCmd when a cmd directory exists
	Decisions string // the decision records directory in use, or the default
	Go        bool   // go.mod exists at the root
}

// Root resolves a path to the real root of a git checkout. It follows the
// symbolic links of the path itself (a temporary directory often is one)
// and refuses a path without a .git entry, so init never writes outside a
// repository.
func Root(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("%w: %s: %w", ErrNotRepository, path, err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("%w: %s: %w", ErrNotRepository, path, err)
	}
	if _, err := os.Stat(filepath.Join(resolved, gitEntry)); err != nil {
		return "", fmt.Errorf("%w: %s has no .git entry", ErrNotRepository, resolved)
	}
	return resolved, nil
}

// Detect reads the layout facts of a root: the main package layout, the
// decision records directory (decision 5: an existing one is reused, two
// is an error) and whether the repository is a Go module.
func Detect(root string) (Layout, error) {
	return ResolveLayout(root, nil, nil)
}

// ResolveLayout detects only absent choices so reviewed settings resolve ambiguity.
func ResolveLayout(root string, main, decisions *string) (Layout, error) {
	directory, err := resolveDecisions(root, decisions)
	if err != nil {
		return Layout{}, err
	}
	return Layout{
		Main: resolveMain(root, main), Decisions: directory, Go: exists(root, goModule),
	}, nil
}

func resolveMain(root string, reviewed *string) string {
	if reviewed != nil {
		return *reviewed
	}
	if !exists(root, rootMain) && exists(root, cmdDir) {
		return MainCmd
	}
	return MainRoot
}

func resolveDecisions(root string, reviewed *string) (string, error) {
	if reviewed != nil {
		return *reviewed, nil
	}
	adr, legacy := exists(root, defaultDecisions), exists(root, legacyDecisions)
	if adr && legacy {
		return "", ErrDecisions
	}
	if legacy {
		return legacyDecisions, nil
	}
	return defaultDecisions, nil
}

// exists reports whether a slash separated relative path exists under root.
func exists(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil
}
