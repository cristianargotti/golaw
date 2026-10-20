package source

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// recordFields are the fields of a record of git ls-files --stage, and of
// git ls-tree, before the tab that opens its path: the mode, the object id
// and the stage, or the mode, the type and the object id.
const recordFields = 3

// IndexOptions says which index a snapshot reads and against which commit
// its changes are taken.
type IndexOptions struct {
	// IndexFile is the absolute path of the index git reads; empty reads the
	// index of the process environment: GIT_INDEX_FILE when git set it for a
	// hook, the repository's own index otherwise.
	IndexFile string
	// Base is the commit the changes are taken against; empty is HEAD, and
	// the empty tree on an unborn branch.
	Base string
}

// Index is the frozen snapshot of an index: the stage 0 entries, classified
// with the classes of section 5.1 (tracked for every regular entry, test,
// fixture, generated and binary from the path and the blob, symlink for
// mode 120000, gitlink for 160000, unreadable for a blob git cannot read),
// each blob read once through git cat-file --batch by its frozen id, within
// model.MaxReadBytes. Its bytes never come from the worktree.
type Index struct {
	*frozen

	identity  string
	unmerged  []string
	changes   []model.Change
	mergeHead string
}

// NewIndex lists the index universe of repo with git ls-files -z --stage,
// freezes the object id of every entry before any blob is read, takes the
// changes against the base with git diff-index --cached -z -M, and reads
// no blob yet. Every git command runs in repo.Root with the environment of
// the process plus GIT_INDEX_FILE when IndexFile is set, so a linked
// worktree and an alternate index are read as git reads them. An
// intent-to-add entry is not part of the universe (the next commit does
// not record it), and a path a merge left at stages 1 to 3 is unmerged,
// outside the universe and the changes.
func NewIndex(ctx context.Context, repo Repo, opts IndexOptions) (*Index, error) {
	var env []string
	if opts.IndexFile != "" {
		env = []string{"GIT_INDEX_FILE=" + opts.IndexFile}
	}
	out, err := gitWith(ctx, repo.Root, env, nil, "ls-files", "-z", "--stage")
	if err != nil {
		return nil, err
	}
	entries, unmerged, err := parseIndex(out)
	if err != nil {
		return nil, err
	}
	changes, intents, err := indexChanges(ctx, repo, env, opts.Base, entries)
	if err != nil {
		return nil, err
	}
	entries = slices.DeleteFunc(entries, func(e treeEntry) bool { return intents[e.path] })
	f := newFrozen(ctx, repo.Root, env, entries)
	merge, err := mergeHead(repo.GitDir)
	if err != nil {
		return nil, err
	}
	first, _, _ := strings.Cut(merge, "\n")
	x := &Index{frozen: f, identity: "index:" + ledger(f.entries), unmerged: unmerged, changes: changes, mergeHead: first}
	return x, nil
}

// Root is the real path of the worktree root.
func (x *Index) Root() string { return x.root }

// Identity names the index a report describes: "index:" followed by the
// sha256 over every path, mode and object id of its universe.
func (x *Index) Identity() string { return x.identity }

// parseIndex reads the records of git ls-files -z --stage: the entries at
// stage 0 are the universe, and a path held at stages 1 to 3 is unmerged,
// named once.
func parseIndex(out []byte) ([]treeEntry, []string, error) {
	var entries []treeEntry
	var unmerged []string
	err := listed(out, "ls-files --stage", func(fields []string, path string) {
		if fields[2] != "0" {
			unmerged = append(unmerged, path)
			return
		}
		entries = append(entries, treeEntry{path: path, mode: fields[0], oid: fields[1]})
	})
	if err != nil {
		return nil, nil, err
	}
	slices.Sort(unmerged)
	return entries, slices.Compact(unmerged), nil
}

// listed reads the NUL separated records a listing of git prints, each
// three fields, a tab and a path, and hands each to keep; a record of
// another shape is ErrGit naming the command.
func listed(out []byte, command string, keep func(fields []string, path string)) error {
	for record := range strings.SplitSeq(strings.TrimSuffix(string(out), "\x00"), "\x00") {
		if record == "" {
			continue
		}
		meta, path, found := strings.Cut(record, "\t")
		fields := strings.Fields(meta)
		if !found || path == "" || len(fields) != recordFields {
			return fmt.Errorf("%w: git %s printed %q", ErrGit, command, record)
		}
		keep(fields, path)
	}
	return nil
}

// ledger is the sha256 over the path, the mode and the object id of every
// entry, in path order: the identity of a frozen universe.
func ledger(entries []treeEntry) string {
	lines := make([]byte, 0, len(entries)*ledgerLine)
	for _, e := range entries {
		lines = append(lines, e.path+"\x00"+e.mode+"\x00"+e.oid+"\n"...)
	}
	return ContentHash(lines)
}
