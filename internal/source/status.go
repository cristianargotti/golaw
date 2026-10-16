package source

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strings"
)

// The fields of a porcelain v2 status record before its path: an ordinary
// change, a rename or copy, whose original path is the next record, and
// an unmerged path.
const (
	ordinaryFields = 9
	renamedFields  = 10
	unmergedFields = 11
)

// Changes asks git status for the paths that differ from HEAD in the
// index, from the index in the worktree, or that git would add, with the
// ignore rules of the universe listing, and answers a snapshot of those
// paths only with every path it reported, a deleted one included. The
// status takes no lock and writes no index (--no-optional-locks). A
// submodule whose only change is the revision it has checked out is left
// out, as its identity is its class; a deleted or replaced one is kept.
func Changes(ctx context.Context, repo Repo) (*Worktree, []string, error) {
	out, err := git(ctx, repo.Root, "--no-optional-locks", "status", "--porcelain=v2", "-z",
		"--untracked-files=all", "--ignore-submodules=dirty")
	if err != nil {
		return nil, nil, err
	}
	entries, err := parseStatus(out)
	if err != nil {
		return nil, nil, err
	}
	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		paths = append(paths, e.path)
	}
	return newWorktree(repo.Root, entries), paths, nil
}

// parseStatus reads the records of git status --porcelain=v2 -z into the
// entries of the paths they name, sorted and unique.
func parseStatus(out []byte) ([]entry, error) {
	records := strings.Split(string(bytes.TrimSuffix(out, []byte{0})), "\x00")
	var entries []entry
	for i := 0; i < len(records); i++ {
		record := records[i]
		kind, _, _ := strings.Cut(record, " ")
		switch kind {
		case "", "#", "!":
			// A header the configuration of the person may add, such as
			// status.showStash, names no path.
			continue
		case "?":
			e := entry{path: strings.TrimPrefix(record, "? ")}
			e.path, e.dir = strings.CutSuffix(e.path, "/")
			entries = append(entries, e)
		case "1", "2", "u":
			found, err := changed(record, kind)
			if err != nil {
				return nil, err
			}
			if kind == "2" && i+1 < len(records) {
				i++
				found = append(found, entry{path: records[i], tracked: true})
			}
			entries = append(entries, found...)
		default:
			return nil, fmt.Errorf("%w: git status printed %q", ErrGit, record)
		}
	}
	slices.SortFunc(entries, func(a, b entry) int { return strings.Compare(a.path, b.path) })
	return slices.CompactFunc(entries, func(a, b entry) bool { return a.path == b.path }), nil
}

// changed reads one record of a tracked path: its letters, its submodule
// field and the path, which may hold spaces, after the fixed fields.
func changed(record, kind string) ([]entry, error) {
	fields := map[string]int{"1": ordinaryFields, "2": renamedFields, "u": unmergedFields}[kind]
	parts := strings.SplitN(record, " ", fields)
	if len(parts) != fields || parts[fields-1] == "" {
		return nil, fmt.Errorf("%w: git status printed %q", ErrGit, record)
	}
	letters, submodule := parts[1], parts[2]
	if strings.HasPrefix(submodule, "S") && strings.Trim(letters, ".M") == "" {
		return nil, nil
	}
	return []entry{{path: parts[fields-1], tracked: true}}, nil
}
