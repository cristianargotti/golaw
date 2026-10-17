package source

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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
// merge-tree --write-tree needs git 2.38; an older git is an error.
func AutomaticMerge(ctx context.Context, repo Repo, ours, theirs string, paths []string) (map[string][]string, error) {
	scratch, err := os.MkdirTemp("", "golaw-merge-")
	if err != nil {
		return nil, fmt.Errorf("%w: a temporary object directory: %w", ErrGit, err)
	}
	defer os.RemoveAll(scratch) //nolint:errcheck // a temporary directory left behind changes no answer
	env, err := scratchObjects(repo.GitDir, scratch)
	if err != nil {
		return nil, err
	}
	tree, err := mergeTree(ctx, repo.Root, env, ours, theirs)
	if err != nil {
		return nil, err
	}
	return mergeBlobs(ctx, repo.Root, env, tree, theirs, paths)
}

// mergeTree is the tree git merge-tree --write-tree writes for the merge of
// two commits, the first field it prints; its exit status 1 is a merge
// with conflicts, whose tree holds them in their markers.
func mergeTree(ctx context.Context, dir string, env []string, ours, theirs string) (string, error) {
	out, err := gitWith(ctx, dir, env, nil, "merge-tree", "--write-tree", "--no-messages", "-z", "--allow-unrelated-histories", ours, theirs)
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
