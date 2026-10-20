package source

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// commitFields are the fields git log writes for a commit in the format of
// Commits before its paths: its name, the second it was committed, the name
// and email of its committer and its parents.
const commitFields = 5

// commitFormat opens each commit with two empty fields, which no list of
// paths holds, so a merge whose paths git opens with an empty field of its
// own is still read field by field.
const commitFormat = "--format=%x00%x00%H%x00%ct%x00%cn%x00%ce%x00%P"

// Commit is one commit git log named: its name, its committer, as git
// writes an identity before the time, the second it was committed, whether
// it is a merge, and the paths it changed: against its parent, a root
// commit against the empty tree, and a merge, the paths it holds apart from
// every parent. Parents are the commits it was made on, in the order git
// records them, none for a root commit.
type Commit struct {
	Name      string
	Committer string
	Time      int64
	Merge     bool
	Paths     []string
	Parents   []string
}

// Commits answers, in one git log, the commits reachable from now and not
// from since, every one reachable from now when since is empty, with the
// paths each changed: a rename is read as its two paths, a root commit
// against the empty tree whatever log.showRoot says, and a merge, as git
// log --cc names them, the paths it holds apart from every parent. A from
// above zero bounds the walk at the commits committed at or after that
// second, so the history of a far checkout is not read; the walk stops at
// the first older commit of each line, as git log --since does.
func Commits(ctx context.Context, repo Repo, since, now string, from int64) ([]Commit, error) {
	args := []string{"log", "-z", "--root", noRenames, "--cc", "--no-color", "--no-show-signature", commitFormat, nameOnly}
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
// Commits: the last two of a run of empty fields open each commit, its five
// fields follow, then each path it changed up to the next empty field.
func parseCommits(out string) ([]Commit, error) {
	fields := strings.Split(out, "\x00")
	var commits []Commit
	for i := 0; ; {
		opened := i
		for i < len(fields) && fields[i] == "" {
			i++
		}
		if i == len(fields) {
			return commits, nil
		}
		if i-opened < 2 || i+commitFields > len(fields) {
			return nil, fmt.Errorf("%w: git log printed %q", ErrGit, fields[i])
		}
		commit, err := commitOf(fields[i : i+commitFields])
		if err != nil {
			return nil, err
		}
		commit.Paths, i = pathsFrom(fields, i+commitFields, commit.Merge)
		commits = append(commits, commit)
	}
}

// commitOf reads the five fields of a commit in the format of Commits.
func commitOf(fields []string) (Commit, error) {
	seconds, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return Commit{}, fmt.Errorf("%w: git log printed the time %q", ErrGit, fields[1])
	}
	parents := strings.Fields(fields[4])
	return Commit{
		Name: fields[0], Committer: fields[2] + " <" + fields[3] + ">", Time: seconds, Merge: len(parents) > 1, Parents: parents,
	}, nil
}

// pathsFrom reads the paths of a commit from the field at i up to the next
// empty field and answers them with the index of that field: git puts a
// newline before the first path of a commit and an empty field before the
// first path of a merge it diffs against every parent.
func pathsFrom(fields []string, i int, merge bool) ([]string, int) {
	if merge && i+1 < len(fields) && fields[i] == "" && fields[i+1] != "" {
		i++
	}
	var paths []string
	for ; i < len(fields) && fields[i] != ""; i++ {
		path := fields[i]
		if len(paths) == 0 {
			path = strings.TrimPrefix(path, "\n")
		}
		paths = append(paths, path)
	}
	return paths, i
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

// Parents answers the commits a commit was made on, in the order git
// records them, none for a root commit, with one git rev-parse.
func Parents(ctx context.Context, repo Repo, commit string) ([]string, error) {
	out, err := git(ctx, repo.Root, "rev-parse", commit+"^@")
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(out)), nil
}
