package source

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// AutomaticMerge answers, for a merge of the commit theirs into the commit
// ours, the blob each path holds in the merge git makes of the two by
// itself and the blob theirs holds there, empty where one holds none: git
// merge-tree --write-tree merges them once, from the merge base it finds,
// a conflict it cannot resolve written in its markers, and git cat-file
// names the blobs of every path in one more question. Both run with a
// temporary object directory in front of the objects of the repository,
// removed after, so the objects of the merge never reach the repository.
// merge-tree --write-tree needs git 2.38; an older git is an error. It is
// AutomaticMergeWith without a strategy option.
func AutomaticMerge(ctx context.Context, repo Repo, ours, theirs string, paths []string) (map[string][]string, error) {
	return AutomaticMergeWith(ctx, repo, ours, theirs, nil, paths)
}

// AutomaticMergeWith is AutomaticMerge with each strategy option passed to
// git merge-tree as -X <option>, so the merge is the one git merge makes
// when its command line gives it those options: with ours or theirs, a
// conflict hunk resolved with that side. An option git does not know is an
// error.
func AutomaticMergeWith(ctx context.Context, repo Repo, ours, theirs string, options, paths []string) (map[string][]string, error) {
	var blobs map[string][]string
	err := withScratch(repo, func(env []string) error {
		tree, err := mergeTree(ctx, repo.Root, env, ours, theirs, options)
		if err == nil {
			blobs, err = mergeBlobs(ctx, repo.Root, env, tree, theirs, paths)
		}
		return err
	})
	return blobs, err
}

// withScratch runs work with the environment that puts a temporary object
// directory in front of the objects of the repository, and removes the
// directory after, so the objects git makes for a merge never reach the
// repository.
func withScratch(repo Repo, work func(env []string) error) error {
	scratch, err := os.MkdirTemp("", "golaw-merge-")
	if err != nil {
		return fmt.Errorf("%w: a temporary object directory: %w", ErrGit, err)
	}
	defer os.RemoveAll(scratch) //nolint:errcheck // a temporary directory left behind changes no answer
	env, err := scratchObjects(repo.GitDir, scratch)
	if err != nil {
		return err
	}
	return work(env)
}

// mergeTree is the tree git merge-tree --write-tree writes for the merge of
// two commits with the strategy options, the first field it prints, git
// reading config before the subcommand, as -c merge.conflictStyle=diff3;
// its exit status 1 is a merge with conflicts, whose tree holds them in
// their markers.
func mergeTree(ctx context.Context, dir string, env []string, ours, theirs string, options []string, config ...string) (string, error) {
	strategy := make([]string, 0, 2*len(options))
	for _, option := range options {
		strategy = append(strategy, "-X", option)
	}
	args := []string{"merge-tree", "--write-tree", "--no-messages", "-z", "--allow-unrelated-histories"}
	out, err := gitWith(ctx, dir, env, nil, slices.Concat(config, args, strategy, []string{ours, theirs})...)
	if failure, failed := errors.AsType[*gitError](err); failed && failure.status == gitAbsent {
		err = nil
	}
	if err != nil {
		return "", err
	}
	tree, _, _ := strings.Cut(string(out), "\x00")
	if tree = strings.TrimSpace(tree); emptyTree(tree) == "" || strings.Trim(tree, "0123456789abcdef") != "" {
		return "", fmt.Errorf("%w: git merge-tree printed %q", ErrGit, tree)
	}
	return tree, nil
}

// mergeBlobs names, with one git cat-file --batch-check, the blob the tree
// of a merge and the commit theirs hold at each path, empty where one holds
// no blob there; a name one line of input cannot carry is left out, so a
// caller reads it as unknown.
func mergeBlobs(ctx context.Context, dir string, env []string, tree, theirs string, paths []string) (map[string][]string, error) {
	var input []byte
	asked := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" || strings.ContainsAny(path, "\n\r") {
			continue
		}
		asked = append(asked, path)
		input = append(input, tree+":"+path+"\n"+theirs+":"+path+"\n"...)
	}
	blobs := make(map[string][]string, len(asked))
	if len(asked) == 0 {
		return blobs, nil
	}
	out, err := gitWith(ctx, dir, env, input, "cat-file", "--batch-check=%(objectname) %(objecttype)")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(lines) != 2*len(asked) {
		return nil, fmt.Errorf("%w: git cat-file named %d of %d objects", ErrGit, len(lines), 2*len(asked))
	}
	for i, path := range asked {
		blobs[path] = []string{blobOf(lines[2*i]), blobOf(lines[2*i+1])}
	}
	return blobs, nil
}

// blobOf is the blob a line of git cat-file names, empty for an object it
// holds none of, a missing one, a directory or a submodule.
func blobOf(line string) string {
	name, kind, found := strings.Cut(line, " ")
	if !found || kind != "blob" {
		return ""
	}
	return name
}

// scratchObjects is the environment that puts a temporary object directory
// in front of the objects of the repository of a git directory: git writes
// the objects it makes there and reads every other one from the repository,
// whose own alternates it follows.
func scratchObjects(gitDir, scratch string) ([]string, error) {
	common := gitDir
	data, err := os.ReadFile(filepath.Join(gitDir, "commondir")) //nolint:gosec // the git directory discovery answered, never a name from an event
	switch {
	case err == nil:
		common = strings.TrimRight(string(data), "\r\n")
		if !filepath.IsAbs(common) {
			common = filepath.Join(gitDir, common)
		}
	case !errors.Is(err, fs.ErrNotExist):
		return nil, fmt.Errorf("%w: read the common directory of %s: %w", ErrGit, gitDir, err)
	}
	objects := filepath.ToSlash(filepath.Join(common, "objects"))
	if strings.ContainsRune(objects, filepath.ListSeparator) || strings.HasPrefix(objects, `"`) {
		objects = `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(objects) + `"`
	}
	return []string{"GIT_OBJECT_DIRECTORY=" + scratch, "GIT_ALTERNATE_OBJECT_DIRECTORIES=" + objects}, nil
}

// Remerged answers, for each merge commit, the paths it holds apart from
// the merge git makes of its parents by itself, a conflict in its markers,
// in one git log --remerge-diff --name-only for all of them; git keeps the
// objects of those merges in a temporary object directory it removes, so
// none reaches the repository. --remerge-diff needs git 2.36; an older git
// is an error.
func Remerged(ctx context.Context, repo Repo, merges []string) (map[string][]string, error) {
	args := []string{"log", "-z", "--no-walk", noRenames, "--remerge-diff", "--no-color", "--no-show-signature", commitFormat, nameOnly}
	out, err := git(ctx, repo.Root, append(append(args, merges...), "--")...)
	if err != nil {
		return nil, err
	}
	commits, err := parseCommits(string(out))
	if err != nil {
		return nil, err
	}
	remerged := make(map[string][]string, len(commits))
	for _, commit := range commits {
		remerged[commit.Name] = commit.Paths
	}
	return remerged, nil
}

// nameOnly has git log and git diff-tree name the paths of a change alone.
const nameOnly = "--name-only"

// RemergedWith answers, for each merge commit, the paths it holds apart
// from the merge git makes of its first two parents by itself with each
// strategy option passed as -X <option>: git merge-tree --write-tree makes
// that merge and git diff-tree names the paths the commit holds apart from
// it, two questions for every merge, with the temporary object directory of
// AutomaticMergeWith, so no object reaches the repository. A merge git
// cannot make, such as one of a commit it does not hold, is an error.
func RemergedWith(ctx context.Context, repo Repo, merges, options []string) (map[string][]string, error) {
	remerged := make(map[string][]string, len(merges))
	err := withScratch(repo, func(env []string) error {
		for _, merge := range merges {
			paths, err := apartFrom(ctx, repo.Root, env, merge, options)
			if err != nil {
				return err
			}
			remerged[merge] = paths
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return remerged, nil
}

// apartFrom names the paths a merge commit holds apart from the merge git
// makes of its first two parents with the strategy options.
func apartFrom(ctx context.Context, dir string, env []string, merge string, options []string) ([]string, error) {
	tree, err := mergeTree(ctx, dir, env, merge+"^1", merge+"^2", options)
	if err != nil {
		return nil, err
	}
	out, err := gitWith(ctx, dir, env, nil, "diff-tree", "-r", "-z", noRenames, nameOnly, tree, merge)
	if err != nil {
		return nil, err
	}
	var paths []string
	for path := range strings.SplitSeq(string(out), "\x00") {
		if path != "" {
			paths = append(paths, path)
		}
	}
	return paths, nil
}
