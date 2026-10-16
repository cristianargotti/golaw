package hook

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// MaxInputBytes bounds allocation before an envelope is trusted.
const MaxInputBytes = 1 << 20

// ReadObject requires EOF so another JSON value cannot hide after the event.
func ReadObject(reader io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, MaxInputBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: read input: %w", model.ErrCannotJudge, err)
	}
	if len(data) > MaxInputBytes {
		return nil, fmt.Errorf("%w: input exceeds 1 MiB", model.ErrCannotJudge)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, fmt.Errorf("%w: invalid JSON object: %w", model.ErrCannotJudge, err)
	}
	if object == nil {
		return nil, fmt.Errorf("%w: input must be one object", model.ErrCannotJudge)
	}
	return data, nil
}

// realLocation resolves the symbolic links of the deepest existing
// ancestor of a path and keeps the rest, so a file in a directory that does
// not exist yet still resolves to where it would land.
func realLocation(path string) string {
	rest := ""
	for dir := filepath.Clean(path); ; {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil { // fallback: a missing directory is kept as given
			return filepath.Join(resolved, rest)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return path
		}
		rest = filepath.Join(filepath.Base(dir), rest)
		dir = parent
	}
}

// enrollmentDir is where the enrollment of an event is resolved (A3): the
// deepest existing directory above the target a write or read names, so
// the repository that holds the file governs it whatever the session's
// working directory is, and a file outside every repository (the agent's
// own plans, a scratch file) is a positively unenrolled location. A shell
// command and an event without a target are judged where they run.
func enrollmentDir(event model.Event) string {
	if event.Target == "" {
		return event.Cwd
	}
	path := filepath.FromSlash(event.Target)
	if !filepath.IsAbs(path) {
		path = filepath.Join(event.Cwd, path)
	}
	for dir := filepath.Dir(filepath.Clean(path)); ; dir = filepath.Dir(dir) {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
		if filepath.Dir(dir) == dir {
			return event.Cwd
		}
	}
}

// Hash binds a decision to bytes without retaining their contents.
func Hash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// Relative keeps candidates inside the discovered repository. The root is
// a real path, so the location is resolved through the symbolic links of
// its directory before the comparison: an agent names its files through
// the working directory it was given, which may be a link to the root.
func Relative(root, cwd, name string) (string, error) {
	if name == "" || !filepath.IsAbs(root) || !filepath.IsAbs(cwd) {
		return "", fmt.Errorf("%w: invalid candidate location", model.ErrCannotJudge)
	}
	path := filepath.FromSlash(name)
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}
	relative, err := filepath.Rel(realLocation(root), realLocation(filepath.Clean(path)))
	if err != nil {
		return "", fmt.Errorf("%w: relative path: %w", model.ErrCannotJudge, err)
	}
	if relative == "." || !filepath.IsLocal(relative) {
		return "", fmt.Errorf("%w: candidate is outside the repository", model.ErrCannotJudge)
	}
	return strings.ReplaceAll(filepath.ToSlash(relative), `\`, "/"), nil
}
