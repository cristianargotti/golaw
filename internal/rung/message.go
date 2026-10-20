package rung

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/source"
)

// The keys of the git configuration the cleanup of a message reads, as git
// config names them, the value that lets git pick the comment character,
// the characters it picks from in order (builtin/commit.c), and the line
// git writes after the comment character above the diff of --verbose.
const (
	cleanupKey       = "commit.cleanup"
	commentCharKey   = "core.commentchar"
	commentStringKey = "core.commentstring"
	autoComment      = "auto"
	autoCandidates   = "#;@!$%^&|:"
	scissorsLine     = " ------------------------ >8 ------------------------"
)

// readCommit reads the message file as git wrote it, with the cleanup, the
// comment string and the author git applies to it.
func readCommit(ctx context.Context, deps Dependencies, file string) (model.Commit, error) {
	message, err := readMessage(deps.Dir, file)
	if err != nil {
		return model.Commit{}, err
	}
	settings, err := gitSettings(ctx, deps)
	if err != nil {
		return model.Commit{}, err
	}
	cleanup := cleanupOf(settings[cleanupKey], deps.Environ)
	comment := settings[commentStringKey]
	if strings.EqualFold(comment, autoComment) {
		comment, message = picked(message, cleanup == model.CleanupStrip)
	}
	return model.Commit{Message: message, Cleanup: cleanup, CommentChar: comment, Author: author(ctx, deps)}, nil
}

// picked reads the comment character git picked under auto from the message
// it wrote, because git picks the first candidate that starts no line of
// the message it prepared, which the hook never sees: the character of its
// scissors line, else the one candidate that starts lines, else the default
// when none does. Lines that start with two or more candidates leave the
// choice unknown, so under strip every such line is left unjudged instead:
// judging a line git drops could refuse a commit git makes.
func picked(message string, strip bool) (string, string) {
	starts := ""
	for line := range strings.SplitSeq(message, "\n") {
		if line == "" || !strings.ContainsRune(autoCandidates, rune(line[0])) {
			continue
		}
		if line[1:] == scissorsLine {
			return line[:1], message
		}
		if !strings.Contains(starts, line[:1]) {
			starts += line[:1]
		}
	}
	if len(starts) <= 1 || !strip {
		return starts[:min(len(starts), 1)], message
	}
	kept := slices.DeleteFunc(strings.Split(message, "\n"), func(line string) bool {
		return line != "" && strings.Contains(starts, line[:1])
	})
	return "", strings.Join(kept, "\n")
}

// readMessage reads the message file git hands the hook, a path relative to
// the worktree root as git names it, within the read bound.
func readMessage(dir, file string) (string, error) {
	path := file
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	f, err := os.Open(path) //nolint:gosec // the message file git hands the hook
	if err != nil {
		return "", fmt.Errorf("read the commit message: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, model.MaxReadBytes+1))
	if err != nil {
		return "", fmt.Errorf("read the commit message: %w", err)
	}
	if len(data) > model.MaxReadBytes {
		return "", fmt.Errorf("%w: the commit message %s", model.ErrTooLarge, file)
	}
	return string(data), nil
}

// gitSettings asks git once for the cleanup mode and the comment string of
// the commit being made, the comment character and the comment string read
// as one setting, the last one git names winning, as git reads them.
func gitSettings(ctx context.Context, deps Dependencies) (map[string]string, error) {
	out, err := plumbing(ctx, deps, "config", "-z", "--get-regexp", `^(commit\.cleanup|core\.comment(char|string))$`)
	if err != nil {
		return nil, err
	}
	if out.Code > 1 {
		return nil, fmt.Errorf("%w: git config: %s", source.ErrGit, strings.TrimSpace(string(out.Stderr)))
	}
	settings := map[string]string{}
	for record := range strings.SplitSeq(string(out.Stdout), "\x00") {
		key, value, _ := strings.Cut(record, "\n")
		if key == commentCharKey {
			key = commentStringKey
		}
		settings[key] = value
	}
	return settings, nil
}

// cleanupOf resolves the cleanup git applies after the hook: the configured
// mode, and for default or none strip when the message was edited and
// whitespace when git ran no editor, which it tells the hook with
// GIT_EDITOR=:.
func cleanupOf(configured string, environ []string) string {
	if configured != "" && configured != "default" {
		return configured
	}
	if editor, _ := variable(environ, "GIT_EDITOR"); editor == ":" {
		return model.CleanupWhitespace
	}
	return model.CleanupStrip
}

// author is the identity the commit is authored under, as git var names it,
// without its time; empty when git cannot name it, which no check needs.
func author(ctx context.Context, deps Dependencies) string {
	out, err := plumbing(ctx, deps, "var", "GIT_AUTHOR_IDENT")
	if err != nil || out.Code != 0 {
		return ""
	}
	ident, _, _ := strings.Cut(string(out.Stdout), "\n")
	ident, _, _ = strings.CutLast(ident, " ")
	ident, _, _ = strings.CutLast(ident, " ")
	return ident
}
