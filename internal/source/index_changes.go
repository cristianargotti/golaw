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
	targets := map[string]bool{}
	for _, c := range x.changes {
		if c.NewPath != "" {
			targets[c.NewPath] = true
		}
	}
	return x.Select(func(path string) bool { return targets[path] })
}

// Unmerged answers the paths held at stages 1 to 3, sorted.
func (x *Index) Unmerged() []string { return slices.Clone(x.unmerged) }

// MergeHead answers the first commit MERGE_HEAD names in the git
// directory, empty when no merge is in progress.
func (x *Index) MergeHead() string { return x.mergeHead }

// indexChanges takes the changes of the universe against the base: git
// diff-index --cached -z -M against a commit, HEAD when no base is named;
// on an unborn branch with no base, every entry is an addition against the
// empty tree and git is not asked.
func indexChanges(ctx context.Context, repo Repo, env []string, base string, entries []treeEntry, intents map[string]bool) ([]model.Change, error) {
	if base == "" {
		base = repo.Head
	}
	if base == "" {
		changes := make([]model.Change, 0, len(entries))
		for _, e := range entries {
			changes = append(changes, model.Change{Op: model.ChangeAdd, NewPath: e.path, Mode: e.mode, After: e.oid})
		}
		return changes, nil
	}
	out, err := gitWith(ctx, repo.Root, env, nil, "diff-index", "--cached", "-z", "-M", "--end-of-options", base, "--")
	if err != nil {
		return nil, err
	}
	changes, err := parseChanges(out, intents)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(changes, func(a, b model.Change) int { return strings.Compare(changeKey(a), changeKey(b)) })
	return changes, nil
}

// parseChanges reads the raw records of git diff-index -z: the two modes,
// the two object ids and the status, then the path, and the new path of a
// rename or a copy.
func parseChanges(out []byte, intents map[string]bool) ([]model.Change, error) {
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
		if change, ok := changeOf(meta, fields[i+1:i+1+paths], intents); ok {
			changes = append(changes, change)
		}
		i += 1 + paths
	}
	return changes, nil
}

// changeOf is the change one raw record stands for: an addition, a copy
// read as the addition of its new path, a modification or a type change,
// a deletion, a rename. An unmerged path is none, Unmerged names it; an
// intent-to-add path is none, because the next commit does not record it,
// and a rename git paired with one is the deletion of its old path.
func changeOf(meta, paths []string, intents map[string]bool) (model.Change, bool) {
	oldMode, newMode, before, after := meta[0], meta[1], meta[2], meta[3]
	switch meta[4][0] {
	case 'A', 'C':
		path := paths[len(paths)-1]
		return model.Change{Op: model.ChangeAdd, NewPath: path, Mode: newMode, After: after}, !intents[path]
	case 'M', 'T':
		return model.Change{Op: model.ChangeModify, OldPath: paths[0], NewPath: paths[0], Mode: newMode, Before: before, After: after}, true
	case 'D':
		return model.Change{Op: model.ChangeDelete, OldPath: paths[0], Mode: oldMode, Before: before}, true
	case 'R':
		if intents[paths[1]] {
			return model.Change{Op: model.ChangeDelete, OldPath: paths[0], Mode: oldMode, Before: before}, true
		}
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
