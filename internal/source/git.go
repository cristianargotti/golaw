package source

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// gitFatal is the exit status git uses for a fatal error, including a
// directory that is not a repository.
const gitFatal = 128

// gitAbsent is the exit status of a plumbing query whose answer is "no":
// an unborn HEAD under rev-parse --verify -q, an unset configuration key.
const gitAbsent = 1

// revParse is named once because the discovery, the parents of a commit
// and the commit a revision resolves to all ask it.
const revParse = "rev-parse"

// Repo is a discovered repository: the real path of its worktree root,
// its git directory, the HEAD commit (empty on an unborn branch) and
// whether the filesystem folds case, as git recorded it at init.
type Repo struct {
	Root       string // the real path of the worktree root, in the OS form
	GitDir     string // the git directory, absolute
	Head       string // the HEAD commit id, empty on an unborn branch
	IgnoreCase bool   // core.ignorecase: the filesystem folds case
}

// Identity is the repository identity the enrollment record is keyed by:
// the real root path in slash form.
func (r Repo) Identity() string { return filepath.ToSlash(r.Root) }

// Discover finds the repository that holds dir with two plumbing
// questions: one rev-parse for the top level, the git directory and HEAD,
// whose verification of HEAD alone exits 1 on an unborn branch, and the
// core.ignorecase setting. A directory git does not recognize, with no
// .git entry in it or above it, is ErrNotRepository; git missing or
// failing otherwise, a repository git cannot read included, is ErrGit; the
// two outcomes of A14 stay distinct.
func Discover(ctx context.Context, dir string) (Repo, error) {
	out, err := git(ctx, dir, revParse, "--show-toplevel", "--absolute-git-dir", "--verify", "-q", "HEAD")
	failure, failed := errors.AsType[*gitError](err)
	if failed && failure.status == gitFatal && outsideCheckout(dir) {
		return Repo{}, fmt.Errorf("%w: %s: %s", ErrNotRepository, dir, failure.reason)
	}
	unborn := failed && failure.status == gitAbsent
	if err != nil && !unborn {
		return Repo{}, err
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lines) != 3 && (!unborn || len(lines) != 2) || lines[0] == "" {
		return Repo{}, fmt.Errorf("%w: rev-parse printed %q", ErrGit, string(out))
	}
	root, err := filepath.EvalSymlinks(filepath.FromSlash(lines[0]))
	if err != nil {
		return Repo{}, fmt.Errorf("%w: resolve %s: %w", ErrGit, lines[0], err)
	}
	repo := Repo{Root: root, GitDir: filepath.FromSlash(lines[1])}
	if !unborn {
		repo.Head = lines[2]
	}
	flag, err := query(ctx, root, "config", "--bool", "core.ignorecase")
	if err != nil {
		return Repo{}, err
	}
	repo.IgnoreCase = flag == "true"
	return repo, nil
}

// outsideCheckout reports whether no directory from dir up to the root of
// the filesystem holds a .git entry. Git exits 128 for every fatal error
// and translates the reason it prints, so the entry git looks for is what
// tells a directory outside every repository from a repository git failed
// on, such as one with an unparsable configuration; an entry that cannot
// be inspected counts as present, because outside is a claim to prove.
func outsideCheckout(dir string) bool {
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return false
	}
	abs, err := filepath.Abs(resolved)
	if err != nil {
		return false
	}
	for current := abs; ; current = filepath.Dir(current) {
		if _, err := os.Lstat(filepath.Join(current, ".git")); !errors.Is(err, fs.ErrNotExist) {
			return false
		}
		if filepath.Dir(current) == current {
			return true
		}
	}
}

// query runs a plumbing question whose "no" is exit status 1 and answers
// the trimmed output, empty for "no".
func query(ctx context.Context, dir string, args ...string) (string, error) {
	out, err := git(ctx, dir, args...)
	if failure, failed := errors.AsType[*gitError](err); failed && failure.status == gitAbsent {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// gitError is a failed plumbing command: its status and git's own words.
type gitError struct {
	args   []string
	status int    // the exit status, 0 when git could not start
	reason string // the first line git printed, or the start failure
}

func (e *gitError) Error() string {
	return fmt.Sprintf("git %s: %s", strings.Join(e.args, " "), e.reason)
}

// Unwrap makes every plumbing failure match ErrGit.
func (e *gitError) Unwrap() error { return ErrGit }

// git runs one plumbing command under dir and returns its stdout, also
// beside the error of a command that failed, whose answer a caller may
// still read. The kit's own plumbing is the only thing ever executed:
// never repository code, never intercepted text.
func git(ctx context.Context, dir string, args ...string) ([]byte, error) {
	return gitInput(ctx, dir, nil, args...)
}

// gitInput is git with the bytes the command reads on its standard input,
// none when input is nil.
func gitInput(ctx context.Context, dir string, input []byte, args ...string) ([]byte, error) {
	return gitWith(ctx, dir, nil, input, args...)
}

// gitWith is gitInput with variables added to the environment git
// inherits, such as the object directory it writes to; the environment of
// the command is the one it would inherit, extended, and no variable of it
// is read here.
func gitWith(ctx context.Context, dir string, env []string, input []byte, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // the arguments are the kit's own plumbing
	cmd.Dir = dir
	if env != nil {
		cmd.Env = append(cmd.Environ(), env...)
	}
	if input != nil {
		cmd.Stdin = bytes.NewReader(input)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err == nil {
		return out, nil
	}
	failure := &gitError{args: args, reason: strings.TrimSpace(stderr.String())}
	if exit, exited := errors.AsType[*exec.ExitError](err); exited {
		failure.status = exit.ExitCode()
	}
	if failure.reason == "" {
		failure.reason = err.Error()
	}
	if line, _, found := strings.Cut(failure.reason, "\n"); found {
		failure.reason = line
	}
	return out, failure
}
