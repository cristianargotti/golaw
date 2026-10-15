package source

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
)

// CleanRel validates a repository-relative path the way the snapshot keys
// it: forward slashes, no leading slash, no empty, dot or dot-dot
// component. Backslashes are read as separators because an agent on
// Windows writes them; a path that would leave the root is ErrEscapedRoot.
func CleanRel(rel string) (string, error) {
	slashed := strings.ReplaceAll(rel, `\`, "/")
	if slashed == "" || strings.HasPrefix(slashed, "/") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("%w: %q is not relative", ErrEscapedRoot, rel)
	}
	cleaned := path.Clean(slashed)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("%w: %q", ErrEscapedRoot, rel)
	}
	return cleaned, nil
}

// OSPath is the filesystem path of a snapshot path under the root: the
// two identities stay apart, the snapshot keys slash separated relative
// paths and the filesystem is spoken to in its own form only here.
func OSPath(root, rel string) string {
	return filepath.Join(root, filepath.FromSlash(rel))
}

// Rel turns a path an agent gave, absolute or relative to dir, into the
// snapshot path under root, which must be a real path. Symbolic links on
// the existing part of the path are resolved, so a path that reaches the
// tree through a link outside it is accepted at its real location, while
// a path whose real location is outside the tree is refused: ErrUnsafeLink
// when a link inside the tree leads out, ErrEscapedRoot otherwise.
func Rel(root, dir, given string) (string, error) {
	if given == "" {
		return "", fmt.Errorf("%w: empty path", ErrEscapedRoot)
	}
	abs := filepath.Clean(given)
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(dir, abs)
	}
	resolved, err := realPath(abs)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, resolved)
	if err == nil && !escapes(rel) {
		return filepath.ToSlash(rel), nil
	}
	if lexical, lerr := filepath.Rel(root, abs); lerr == nil && !escapes(lexical) {
		return "", fmt.Errorf("%w: %s resolves to %s", ErrUnsafeLink, given, resolved)
	}
	return "", fmt.Errorf("%w: %s", ErrEscapedRoot, given)
}

// escapes reports whether a relative path is the root itself or above it.
func escapes(rel string) bool {
	return rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// realPath resolves the symbolic links of the longest existing prefix of
// abs and appends the rest, so a path about to be created resolves too.
func realPath(abs string) (string, error) {
	rest := ""
	for current := abs; ; {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			return filepath.Join(resolved, rest), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("resolve %s: %w", abs, err)
		}
		parent, name := filepath.Split(current)
		parent = filepath.Clean(parent)
		if parent == current {
			return "", fmt.Errorf("resolve %s: %w", abs, err)
		}
		rest = filepath.Join(name, rest)
		current = parent
	}
}
