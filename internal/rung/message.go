package rung

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/source"
)

// The keys of the git configuration the cleanup of a message reads, as git
// config names them, and the comment string git uses when auto picks one.
const (
	cleanupKey       = "commit.cleanup"
	commentCharKey   = "core.commentchar"
	commentStringKey = "core.commentstring"
	autoComment      = "auto"
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
	comment := settings[commentStringKey]
	if comment == autoComment {
		comment = ""
	}
	return model.Commit{
		Message: message, Cleanup: cleanupOf(settings[cleanupKey], deps.Environ),
		CommentChar: comment, Author: author(ctx, deps),
	}, nil
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
