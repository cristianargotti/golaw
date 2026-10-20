package source

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// Changes answers the staged changes against the base, in path order, a
// deletion at its old path; a rename carries both paths. The slice is a
// copy.
func (x *Index) Changes() []model.Change { return slices.Clone(x.changes) }

// Staged answers the files the changes add, modify or rename to, in path
// order, classified as the universe classifies them.
func (x *Index) Staged() []model.File {
	targets := newPaths(x.changes)
	return x.Select(func(path string) bool { return targets[path] })
}

// Unmerged answers the paths held at stages 1 to 3, sorted.
func (x *Index) Unmerged() []string { return slices.Clone(x.unmerged) }

// MergeHead answers the first commit MERGE_HEAD names in the git
// directory, empty when no merge is in progress.
func (x *Index) MergeHead() string { return x.mergeHead }

// indexChanges takes the changes the next commit records against the base
// and names the intent-to-add entries of the listing. The base is the one
// named, else HEAD, else on an unborn branch the empty tree, and the
// changes are one git diff-index --cached -z -M --ita-invisible-in-index
// against it, which reads the index as the commit writes it: an intent to
// add is no addition, and the deletion of a path the base holds. On an
// unborn branch where no entry holds the empty blob, no entry can be an
// intent, every entry is an addition and git is not asked.
func indexChanges(ctx context.Context, repo Repo, env []string, base string, entries []treeEntry) ([]model.Change, map[string]bool, error) {
	if base == "" {
		base = repo.Head
	}
	empty := slices.IndexFunc(entries, holdsEmptyBlob)
	if base == "" && empty < 0 {
		changes := make([]model.Change, 0, len(entries))
		for _, e := range entries {
			changes = append(changes, model.Change{Op: model.ChangeAdd, NewPath: e.path, Mode: e.mode, After: e.oid})
		}
		return changes, nil, nil
	}
	against := base
	if against == "" {
		against = emptyTree(entries[empty].oid)
	}
	changes, err := diffIndex(ctx, repo.Root, env, against, "--ita-invisible-in-index")
	if err != nil {
		return nil, nil, err
	}
	if empty < 0 {
		return changes, nil, nil
	}
	intents, err := intentsToAdd(ctx, repo.Root, env, base, entries, changes)
	if err != nil {
		return nil, nil, err
	}
	return changes, intents, nil
}

// intentsToAdd names the entries git add -N recorded, from the index alone
// and never the worktree, so an intent whose file left the worktree is
// still one. An intent holds the empty blob, and the changes, which leave
// intents out, never name it as a new path. An empty entry they do not
// name as new is an intent when the diff that shows intents names it as
// new, where the base lacks it, or when the changes delete or rename it,
// where the base holds it; otherwise it is an empty file the base already
// holds. On an unborn branch every entry is new against the empty tree, so
// that second diff is not taken.
func intentsToAdd(ctx context.Context, root string, env []string, base string, entries []treeEntry, changes []model.Change) (map[string]bool, error) {
	var shown map[string]bool
	if base != "" {
		visible, err := diffIndex(ctx, root, env, base, "--ita-visible-in-index")
		if err != nil {
			return nil, err
		}
		shown = newPaths(visible)
	}
	kept, left := newPaths(changes), map[string]bool{}
	for _, c := range changes {
		if c.Op == model.ChangeDelete || c.Op == model.ChangeRename {
			left[c.OldPath] = true
		}
	}
	intents := map[string]bool{}
	for _, e := range entries {
		if holdsEmptyBlob(e) && !kept[e.path] && (base == "" || shown[e.path] || left[e.path]) {
			intents[e.path] = true
		}
	}
	return intents, nil
}

// diffIndex takes one git diff-index --cached -z -M of the index against a
// tree, with the option that shows or hides the intent-to-add entries, and
// answers its changes in path order; --end-of-options makes git read the
// base as a revision and never as an option.
func diffIndex(ctx context.Context, root string, env []string, base, intents string) ([]model.Change, error) {
	out, err := gitWith(ctx, root, env, nil, "diff-index", "--cached", "-z", "-M", intents, "--end-of-options", base, "--")
	if err != nil {
		return nil, err
	}
	changes, err := parseChanges(out)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(changes, func(a, b model.Change) int { return strings.Compare(changeKey(a), changeKey(b)) })
	return changes, nil
}

// parseChanges reads the raw records of git diff-index -z: the two modes,
// the two object ids and the status, then the path, and the new path of a
// rename or a copy.
func parseChanges(out []byte) ([]model.Change, error) {
	fields := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	var changes []model.Change
	for i := 0; i+1 < len(fields); {
		meta := strings.Fields(strings.TrimPrefix(fields[i], ":"))
		if len(meta) != rawFields {
			return nil, fmt.Errorf("%w: git diff-index printed %q", ErrGit, fields[i])
		}
		paths := 1
		if meta[4][0] == 'R' || meta[4][0] == 'C' {
			paths = 2
		}
		if i+paths >= len(fields) {
			return nil, fmt.Errorf("%w: git diff-index printed %q without its paths", ErrGit, fields[i])
		}
		if change, ok := changeOf(meta, fields[i+1:i+1+paths]); ok {
			changes = append(changes, change)
		}
		i += 1 + paths
	}
	return changes, nil
}

// changeOf is the change one raw record stands for: an addition, a copy
// read as the addition of its new path, a modification or a type change,
// a deletion, a rename. An unmerged path is none, Unmerged names it.
func changeOf(meta, paths []string) (model.Change, bool) {
	oldMode, newMode, before, after := meta[0], meta[1], meta[2], meta[3]
	switch meta[4][0] {
	case 'A', 'C':
		return model.Change{Op: model.ChangeAdd, NewPath: paths[len(paths)-1], Mode: newMode, After: after}, true
	case 'M', 'T':
		return model.Change{Op: model.ChangeModify, OldPath: paths[0], NewPath: paths[0], Mode: newMode, Before: before, After: after}, true
	case 'D':
		return model.Change{Op: model.ChangeDelete, OldPath: paths[0], Mode: oldMode, Before: before}, true
	case 'R':
		return model.Change{Op: model.ChangeRename, OldPath: paths[0], NewPath: paths[1], Mode: newMode, Before: before, After: after}, true
	default:
		return model.Change{}, false
	}
}

// changeKey is the path a change is ordered by: its new path, or the old
// path of a deletion.
func changeKey(c model.Change) string {
	if c.NewPath != "" {
		return c.NewPath
	}
	return c.OldPath
}

// newPaths is the set of the paths the changes add, modify or rename to.
func newPaths(changes []model.Change) map[string]bool {
	paths := make(map[string]bool, len(changes))
	for _, c := range changes {
		if c.NewPath != "" {
			paths[c.NewPath] = true
		}
	}
	return paths
}

// holdsEmptyBlob reports whether an entry holds the empty blob, the one an
// intent-to-add entry records, so only such an entry can be an intent.
func holdsEmptyBlob(e treeEntry) bool { return e.oid == emptyBlobSHA1 || e.oid == emptyBlobSHA256 }
