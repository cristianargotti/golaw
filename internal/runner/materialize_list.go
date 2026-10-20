package runner

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// maxTreeRecord exceeds the path length of every supported host, so a
// record is read whole without buffering an unbounded one.
const maxTreeRecord = 128 << 10

func listTree(ctx context.Context, in MaterializeInput) ([]treeEntry, error) {
	args, err := treeListArgs(ctx, in)
	if err != nil {
		return nil, err
	}
	var entries []treeEntry
	consume := func(reader io.Reader) error {
		var readErr error
		entries, readErr = readTreeEntries(reader, in.Rev != "", in.Bounds.Files)
		return readErr
	}
	out, err := ReadOutput(ctx, treeCommand(in, args...), consume)
	if err != nil {
		return nil, fmt.Errorf("list materialized tree: %w", err)
	}
	if out.Code != 0 {
		return nil, treeGitError(args, out)
	}
	if in.Rev != "" {
		return entries, nil
	}
	return leaveIntents(ctx, in, entries)
}

func treeListArgs(ctx context.Context, in MaterializeInput) ([]string, error) {
	if in.Rev == "" {
		return []string{"ls-files", "--stage", "-z"}, nil
	}
	args := []string{"rev-parse", "--verify", "--quiet", "--end-of-options", in.Rev + "^{tree}"}
	out, err := Run(ctx, treeCommand(in, args...))
	if err != nil {
		return nil, fmt.Errorf("resolve materialization revision: %w", err)
	}
	object := strings.TrimSpace(string(out.Stdout))
	switch {
	case out.Code == 1 && object == "":
		return nil, fmt.Errorf("%w: revision %q has no local tree", model.ErrCannotJudge, in.Rev)
	case out.Code != 0:
		return nil, treeGitError(args, out)
	case !validObject(object) || zeroObject(object):
		return nil, fmt.Errorf("%w: git rev-parse answered %q for revision %q", model.ErrCannotJudge, object, in.Rev)
	}
	return []string{"ls-tree", "-r", "-z", object}, nil
}

func readTreeEntries(reader io.Reader, revision bool, limit int) ([]treeEntry, error) {
	buffer := bufio.NewReaderSize(reader, maxTreeRecord)
	entries := make([]treeEntry, 0)
	for {
		record, err := buffer.ReadSlice(0)
		if errors.Is(err, io.EOF) && len(record) == 0 {
			return entries, validateTreeHierarchy(entries)
		}
		if err != nil {
			return nil, fmt.Errorf("%w: read tree entry: %w", model.ErrCannotJudge, err)
		}
		if len(entries) >= limit {
			return nil, fmt.Errorf("%w: tree has %d entries, limit %d", ErrBound, len(entries)+1, limit)
		}
		entry, err := parseTreeEntry(string(record[:len(record)-1]), revision)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
}

func parseTreeEntry(record string, revision bool) (treeEntry, error) {
	header, name, found := strings.Cut(record, "\t")
	fields := strings.Fields(header)
	const treeFields = 3
	if !found || len(fields) != treeFields {
		return treeEntry{}, fmt.Errorf("%w: invalid tree entry %q", model.ErrCannotJudge, record)
	}
	entry := treeEntry{path: name, mode: fields[0], object: fields[1]}
	if revision {
		entry.object = fields[2]
		wantType := "blob"
		if entry.mode == "160000" {
			wantType = "commit"
		}
		if fields[1] != wantType {
			return treeEntry{}, fmt.Errorf("%w: invalid object type for %q", model.ErrCannotJudge, name)
		}
	} else if fields[2] != "0" {
		return treeEntry{}, fmt.Errorf("%w: unmerged index path %q at stage %q", model.ErrCannotJudge, name, fields[2])
	}
	if !validObject(entry.object) || zeroObject(entry.object) || !treeMode(entry.mode) {
		return treeEntry{}, fmt.Errorf("%w: invalid mode or object for %q", model.ErrCannotJudge, name)
	}
	return entry, validateTreePath(name)
}

func treeMode(mode string) bool {
	return mode == "100644" || mode == "100755" || mode == "120000" || mode == "160000"
}

// validateTreePath refuses a path that leaves the tree or names git
// metadata under any spelling a host folds to .git. A colon is left to
// filepath.IsLocal, which refuses it on Windows, where it names a drive or
// a stream, while elsewhere it is part of a name such as a timestamp.
func validateTreePath(name string) error {
	if !filepath.IsLocal(name) || strings.HasPrefix(name, "\\") {
		return fmt.Errorf("%w: rooted tree path %q", model.ErrCannotJudge, name)
	}
	for component := range strings.SplitSeq(filepath.ToSlash(name), "/") {
		alias := strings.TrimRight(component, " .")
		if component == "" || component == "." || component == ".." ||
			strings.EqualFold(alias, ".git") || strings.EqualFold(alias, "git~1") {
			return fmt.Errorf("%w: unsafe tree path %q", model.ErrCannotJudge, name)
		}
	}
	return nil
}

func validateTreeHierarchy(entries []treeEntry) error {
	names := make(map[string]bool, len(entries))
	for i := range entries {
		name := entries[i].path
		if names[name] {
			return fmt.Errorf("%w: duplicate tree path %q", model.ErrCannotJudge, name)
		}
		names[name] = true
	}
	for i := range entries {
		for parent := path.Dir(entries[i].path); parent != "."; parent = path.Dir(parent) {
			if names[parent] {
				return fmt.Errorf("%w: tree entry is also a parent %q", model.ErrCannotJudge, parent)
			}
		}
	}
	return nil
}

// emptyObjects answers the empty blob, then the empty tree, of the object
// format an object name spells: git add -N records the empty blob, and the
// index compared with the empty tree names every path the next commit
// records.
func emptyObjects(object string) (string, string) {
	if len(object) == sha256HexLength {
		return "473a0f4c3be8a93681a267e3b1e9a7dcda1185436fe141f7749120a303721813",
			"6ef19b41225c5369f1c104d45d8d85efa9b057b53b14b4b9b939dd74decc5321"
	}
	return "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391", "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
}

// leaveIntents drops the entries git add -N recorded, because the next
// commit writes none of them and a copy that kept one would hand the tools
// an empty file. An intent holds the empty blob, so git is asked only when
// an entry does, and the paths it then names against the empty tree with
// intents hidden are those the commit records.
func leaveIntents(ctx context.Context, in MaterializeInput, entries []treeEntry) ([]treeEntry, error) {
	empty, tree := map[string]bool{}, ""
	for i := range entries {
		blob, emptyTree := emptyObjects(entries[i].object)
		if entries[i].mode != "160000" && entries[i].object == blob {
			empty[entries[i].path], tree = true, emptyTree
		}
	}
	if len(empty) == 0 {
		return entries, nil
	}
	args := []string{"diff-index", "--cached", "-z", "--name-only", "--no-renames", "--ita-invisible-in-index", tree, "--"}
	recorded := map[string]bool{}
	out, err := ReadOutput(ctx, treeCommand(in, args...), func(reader io.Reader) error {
		return readRecordedPaths(reader, empty, recorded)
	})
	if err != nil {
		return nil, fmt.Errorf("find the intents to add: %w", err)
	}
	if out.Code != 0 {
		return nil, treeGitError(args, out)
	}
	return slices.DeleteFunc(entries, func(entry treeEntry) bool {
		return empty[entry.path] && !recorded[entry.path]
	}), nil
}

// readRecordedPaths keeps, of the NUL separated paths the diff names, those
// that hold the empty blob.
func readRecordedPaths(reader io.Reader, empty, recorded map[string]bool) error {
	buffer := bufio.NewReaderSize(reader, maxTreeRecord)
	for {
		record, err := buffer.ReadSlice(0)
		if errors.Is(err, io.EOF) && len(record) == 0 {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%w: read a recorded path: %w", model.ErrCannotJudge, err)
		}
		if name := string(record[:len(record)-1]); empty[name] {
			recorded[name] = true
		}
	}
}
