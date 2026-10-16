package shell

import (
	"bytes"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// The theme a non-feat commit may carry: at most this many staged files and
// this many new lines.
const (
	themeFiles = 40
	themeLines = 400
)

// subjectType reads the conventional type of a subject line.
var subjectType = regexp.MustCompile(`^\s*([a-z]+)(?:\([^)]*\))?!?:`)

// Shape is shell.shape: a commit whose type is not feat carries one theme.
// The shape is measured through the snapshot from what the same command
// line stages with git add: every named path counts as a staged file, a
// directory, "." or -A counts the untracked files beneath it, and the lines
// of the untracked files count as new lines; modifications of tracked files
// are measured by the commit rung of M2 over the index.
func Shape() model.Check {
	return commandRule{spec: spec("shell.shape"), judge: judgeShape}
}

func judgeShape(in model.Input, segs []segment) ([]model.Finding, error) {
	kind, ok := commitKind(segs)
	if !ok || kind == "feat" {
		return nil, nil
	}
	files, lines, unread, err := staged(segs, in.Snapshot)
	if err != nil {
		return nil, err
	}
	if files <= themeFiles && lines <= themeLines {
		if len(unread) > 0 {
			// The lines of a file over the read bound may break the theme.
			return nil, &unreadError{paths: unread}
		}
		return nil, nil
	}
	if kind == "" {
		kind = "typeless"
	}
	message := fmt.Sprintf("a %s commit stages %d files and %d new lines; split it by theme or name it feat (limits %d files, %d lines)",
		kind, files, lines, themeFiles, themeLines)
	return []model.Finding{deny("commit-shape", message)}, nil
}

// commitKind returns the conventional type of the first commit the command
// line creates, empty when its subject has none, and false without a commit.
func commitKind(segs []segment) (string, bool) {
	for _, call := range gitCalls(segs) {
		if !isCommit(call) {
			continue
		}
		subject, _, _ := strings.Cut(messageOf(call), "\n")
		if m := subjectType.FindStringSubmatch(subject); m != nil {
			return m[1], true
		}
		return "", true
	}
	return "", false
}

// staged counts the files the command line stages and the new lines they
// carry, each path once, with the files over the read bound whose lines
// it could not count.
func staged(segs []segment, snap model.Snapshot) (int, int, []string, error) {
	seen, lines := map[string]bool{}, 0
	var unread []string
	for _, call := range gitCalls(segs) {
		if call.sub != "add" {
			continue
		}
		for _, named := range addPaths(call.args) {
			added, over, err := newLines(expand(named, snap), snap, seen)
			if err != nil {
				return 0, 0, nil, err
			}
			lines += added
			unread = append(unread, over...)
		}
	}
	return len(seen), lines, unread, nil
}

// newLines records every file of the list as staged and counts the lines
// of the untracked ones, each path once; a file over the read bound counts
// as staged and is answered aside, its lines uncounted.
func newLines(files []model.File, snap model.Snapshot, seen map[string]bool) (int, []string, error) {
	lines := 0
	var unread []string
	for _, f := range files {
		if seen[f.Path] {
			continue
		}
		seen[f.Path] = true
		if f.Class != model.ClassUntracked {
			continue
		}
		content, err := snap.Read(f.Path)
		if errors.Is(err, model.ErrTooLarge) {
			unread = append(unread, f.Path)
			continue
		}
		if err != nil {
			return 0, nil, fmt.Errorf("read %s: %w", f.Path, err)
		}
		lines += lineCount(content)
	}
	return lines, unread, nil
}

// unreadError is the answer of a rule that a file over the read bound left
// open: the files it could not read, which the rule names as unread.
type unreadError struct{ paths []string }

func (e *unreadError) Error() string { return "over the read bound: " + strings.Join(e.paths, ", ") }

// Unwrap makes the answer match the read bound it comes from.
func (e *unreadError) Unwrap() error { return model.ErrTooLarge }

// addPaths returns the paths a git add call names, cleaned; -A and --all
// stand for the whole tree and the other options are skipped.
func addPaths(args []string) []string {
	var out []string
	for _, a := range args {
		switch {
		case a == "-A" || a == "--all":
			out = append(out, ".")
		case strings.HasPrefix(a, "-"):
			continue
		default:
			out = append(out, path.Clean(a))
		}
	}
	return out
}

// expand lists what a named path stands for: the untracked files under a
// directory, or the named file itself with the class the snapshot gives it.
// Only the files under the named path are asked for.
func expand(named string, snap model.Snapshot) []model.File {
	prefix := named + "/"
	if named == "." {
		prefix = ""
	}
	under := model.Select(snap, func(p string) bool { return strings.HasPrefix(p, prefix) })
	if named != "." && len(under) == 0 {
		return []model.File{{Path: named, Class: snap.Class(named)}}
	}
	var out []model.File
	for _, f := range under {
		if f.Class == model.ClassUntracked {
			out = append(out, f)
		}
	}
	return out
}

// lineCount counts the lines of a file the way wc does, plus one for an
// unterminated last line.
func lineCount(content []byte) int {
	n := bytes.Count(content, []byte("\n"))
	if len(content) > 0 && content[len(content)-1] != '\n' {
		n++
	}
	return n
}
