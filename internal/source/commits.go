package source

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// commitFields are the fields git log writes for a commit in the format of
// Commits before its paths: the empty field that opens the commit, its
// name, the second it was committed, and the name and email of its
// committer.
const commitFields = 5

// Commit is one commit git log named: its committer, as git writes an
// identity before the time, the second it was committed and the paths it
// changed against its parent, a root commit against the empty tree.
type Commit struct {
	Committer string
	Time      int64
	Paths     []string
}

// Commits answers, in one git log, the commits reachable from now and not
// from since, every one reachable from now when since is empty, that are no
// merge, with the paths each changed: a rename is read as its two paths and
// a root commit against the empty tree whatever log.showRoot says. A from
// above zero bounds the walk at the commits committed at or after that
// second, so the history of a far checkout is not read; the walk stops at
// the first older commit of each line, as git log --since does.
func Commits(ctx context.Context, repo Repo, since, now string, from int64) ([]Commit, error) {
	args := []string{
		"log", "-z", "--root", "--no-renames", "--no-merges", "--no-color", "--no-show-signature",
		"--format=%x00%H%x00%ct%x00%cn%x00%ce", "--name-only",
	}
	if from > 0 {
		args = append(args, "--since=@"+strconv.FormatInt(from, 10))
	}
	revision := now
	if since != "" {
		revision = since + ".." + now
	}
	out, err := git(ctx, repo.Root, append(args, revision, "--")...)
	if err != nil {
		return nil, err
	}
	return parseCommits(string(out))
}

// parseCommits reads the records git log -z writes in the format of
// Commits: an empty field opens each commit and its four fields follow,
// then each path it changed up to the empty field of the next one; git puts
// a newline between the fields and the first path.
func parseCommits(out string) ([]Commit, error) {
	if out == "" {
		return nil, nil
	}
	fields := strings.Split(strings.TrimSuffix(out, "\x00"), "\x00")
	var commits []Commit
	for i := 0; i < len(fields); {
		if fields[i] != "" || i+commitFields > len(fields) {
			return nil, fmt.Errorf("%w: git log printed %q", ErrGit, fields[i])
		}
		seconds, err := strconv.ParseInt(fields[i+2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%w: git log printed the time %q", ErrGit, fields[i+2])
		}
		commit := Commit{Committer: fields[i+3] + " <" + fields[i+4] + ">", Time: seconds}
		for i += commitFields; i < len(fields) && fields[i] != ""; i++ {
			path := fields[i]
			if len(commit.Paths) == 0 {
				path = strings.TrimPrefix(path, "\n")
			}
			commit.Paths = append(commit.Paths, path)
		}
		commits = append(commits, commit)
	}
	return commits, nil
}

// Committer answers the identity git commits as in the repository: the
// name and email git var GIT_COMMITTER_IDENT writes before the time.
func Committer(ctx context.Context, repo Repo) (string, error) {
	out, err := git(ctx, repo.Root, "var", "GIT_COMMITTER_IDENT")
	if err != nil {
		return "", err
	}
	ident := strings.TrimSpace(string(out))
	end := strings.LastIndex(ident, ">")
	if end < 0 {
		return "", fmt.Errorf("%w: git var printed %q", ErrGit, ident)
	}
	return ident[:end+1], nil
}
