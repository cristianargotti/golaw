package install

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// excludeLine is the pattern a Claude project installation adds to the
// exclude file of the repository, anchored at its root, so git ignores the
// settings.local.json it writes there as Claude Code ignores the one it
// creates itself.
const excludeLine = "/.claude/settings.local.json"

// Exclude is the line a project installation added to the exclude file of
// the repository's git directory: info/exclude, which is never committed
// and serves that clone alone. Created says the installation created the
// file.
type Exclude struct {
	Path    string `json:"path"`
	Line    string `json:"line"`
	Created bool   `json:"created"`
}

// addExclude makes git ignore the settings file of a project installation,
// whose entries name a path under the person's state directory: a scan of
// the repository and a git add then never see it. A line the person
// already wrote stays theirs; a line an earlier install added stays owned
// and comes back if it was removed. A root without .git has nothing to
// ignore.
func addExclude(plan *Plan, previous Receipt) error {
	if plan.Receipt.Scope != scopeProject {
		return nil
	}
	root := filepath.Dir(filepath.Dir(plan.Receipt.SettingsPath))
	path, found, err := gitExcludeFile(root)
	if err != nil || !found {
		return err
	}
	current, exists, err := readOptional(path)
	if err != nil {
		return err
	}
	owned := previous.Exclude != nil && previous.Exclude.Path == path
	present := hasLine(current, excludeLine)
	if present && !owned {
		return nil
	}
	record := Exclude{Path: path, Line: excludeLine, Created: !exists}
	if owned {
		record.Created = previous.Exclude.Created
	}
	plan.Receipt.Exclude = &record
	if present {
		return nil
	}
	return addChange(plan, path, appendLine(current, excludeLine), false)
}

// removeExclude takes out the line the installation added once the
// settings file is gone after the uninstall; while the file still holds
// entries of the person it stays ignored and the line stays with it. The
// exclude file goes when the installation created it and nothing else is
// left.
func removeExclude(plan *Plan, gone bool) error {
	record := plan.Receipt.Exclude
	if record == nil || !gone {
		return nil
	}
	if record.Line != excludeLine || filepath.Base(record.Path) != "exclude" ||
		filepath.Base(filepath.Dir(record.Path)) != "info" {
		return fmt.Errorf("receipt exclude %s: %w", record.Path, ErrPath)
	}
	current, exists, err := readOptional(record.Path)
	if err != nil || !exists {
		return err
	}
	rest, found := withoutLine(current, record.Line)
	if !found {
		return nil
	}
	if record.Created && len(bytes.TrimSpace(rest)) == 0 {
		return addChange(plan, record.Path, nil, true)
	}
	return addChange(plan, record.Path, rest, false)
}

// gitExcludeFile is the exclude file git reads for the checkout at root:
// info/exclude of its common git directory, which a linked worktree shares
// with its main checkout. A root without .git has none.
func gitExcludeFile(root string) (string, bool, error) {
	dotGit := filepath.Join(root, ".git")
	info, err := os.Stat(dotGit)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("inspect %s: %w", dotGit, err)
	}
	dir := dotGit
	if !info.IsDir() {
		if dir, err = gitPointer(dotGit, "gitdir:", root); err != nil {
			return "", false, err
		}
	}
	common, err := gitPointer(filepath.Join(dir, "commondir"), "", dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		common = dir
	case err != nil:
		return "", false, err
	}
	return filepath.Join(common, "info", "exclude"), true, nil
}

// gitPointer reads the directory a git pointer file names after its
// prefix, relative to base when the file gives no absolute path.
func gitPointer(path, prefix, base string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the git pointer files of the installation root
	if err != nil {
		return "", fmt.Errorf("read the git pointer: %w", err)
	}
	value, found := strings.CutPrefix(strings.TrimSpace(string(data)), prefix)
	value = strings.TrimSpace(value)
	if !found || value == "" {
		return "", fmt.Errorf("git pointer %s: %w", path, ErrPath)
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(base, value)
	}
	return filepath.Clean(value), nil
}

// hasLine reports whether text holds line as one of its lines, a carriage
// return before the line break ignored.
func hasLine(text []byte, line string) bool {
	_, found := withoutLine(text, line)
	return found
}

// appendLine adds line at the end of text, after a line break when text
// does not end with one.
func appendLine(text []byte, line string) []byte {
	out := bytes.Clone(text)
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	return append(out, line+"\n"...)
}

// withoutLine answers text without the first line equal to line, its line
// break included, and whether one was found.
func withoutLine(text []byte, line string) ([]byte, bool) {
	start := 0
	for start < len(text) {
		end := bytes.IndexByte(text[start:], '\n')
		next := len(text)
		if end >= 0 {
			next = start + end + 1
		}
		if string(bytes.TrimRight(text[start:next], "\r\n")) == line {
			return append(bytes.Clone(text[:start]), text[next:]...), true
		}
		start = next
	}
	return text, false
}
