package source

import (
	"context"
	"fmt"
)

// emptyRevision is the identity of the empty tree, the base of every file
// on an unborn branch.
const emptyRevision = "tree:empty"

// Revision is the frozen snapshot of the tree of one commit.
type Revision struct {
	*frozen

	identity string
}

// NewRevision is the snapshot of the tree a commit records, listed with git
// ls-tree -r -z and read by frozen object id, with the classes of the index
// snapshot. An empty rev is the empty tree: a snapshot without files. A
// name that resolves to no commit is ErrGit, and so is a name git would
// read as an option, which reaches git only as a revision.
func NewRevision(ctx context.Context, repo Repo, rev string) (*Revision, error) {
	if rev == "" {
		return &Revision{frozen: newFrozen(ctx, repo.Root, nil, nil), identity: emptyRevision}, nil
	}
	commit, err := resolveCommit(ctx, repo.Root, rev)
	if err != nil {
		return nil, err
	}
	out, err := git(ctx, repo.Root, "ls-tree", "-r", "-z", "--full-tree", commit)
	if err != nil {
		return nil, err
	}
	var entries []treeEntry
	err = listed(out, "ls-tree", func(fields []string, path string) {
		entries = append(entries, treeEntry{path: path, mode: fields[0], oid: fields[2]})
	})
	if err != nil {
		return nil, err
	}
	return &Revision{frozen: newFrozen(ctx, repo.Root, nil, entries), identity: "tree:" + commit}, nil
}

// Identity names the tree a report describes: "tree:" followed by the
// commit id, "tree:empty" for the empty tree.
func (r *Revision) Identity() string { return r.identity }

// resolveCommit is the commit a name resolves to. A name that resolves to
// no commit, a tree included, is ErrGit, and so is a name git would read
// as an option, which reaches git only as a revision.
func resolveCommit(ctx context.Context, root, rev string) (string, error) {
	commit, err := query(ctx, root, revParse, "--verify", "-q", "--end-of-options", rev+"^{commit}")
	if err != nil {
		return "", err
	}
	if commit == "" {
		return "", fmt.Errorf("%w: %q names no commit", ErrGit, rev)
	}
	return commit, nil
}
