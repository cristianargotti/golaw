package source

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
)

// The fields of a porcelain v2 status record before its path: an ordinary
// change, a rename or copy, whose original path is the next record, and
// an unmerged path; and the field of an ordinary or renamed record that
// names the object HEAD holds.
const (
	ordinaryFields = 9
	renamedFields  = 10
	unmergedFields = 11
	headField      = 6
)

// Status is what one git status reported: a snapshot of the paths it
// named and of the paths asked for beside them, the paths it named, a
// deleted one included, and the blob HEAD holds at each tracked path it
// named, so a path whose status changed can be compared by its bytes.
type Status struct {
	Worktree *Worktree
	Paths    []string
	Heads    map[string]string
}

// Changes asks git status for the paths that differ from HEAD in the
// index, from the index in the worktree, or that git would add, with the
// ignore rules of the universe listing, and answers them with a snapshot
// that also reads the paths of also, as they are now. The status takes no
// lock and writes no index (--no-optional-locks). A submodule whose only
// change is the revision it has checked out is left out, as its identity
// is its class; a deleted or replaced one is kept.
func Changes(ctx context.Context, repo Repo, also []string) (Status, error) {
	out, err := git(ctx, repo.Root, "--no-optional-locks", "status", "--porcelain=v2", "-z",
		"--untracked-files=all", "--ignore-submodules=dirty")
	if err != nil {
		return Status{}, err
	}
	entries, err := parseStatus(out)
	if err != nil {
		return Status{}, err
	}
	st := Status{Paths: make([]string, 0, len(entries)), Heads: map[string]string{}}
	for _, e := range entries {
		st.Paths = append(st.Paths, e.path)
		if e.head != "" {
			st.Heads[e.path] = e.head
		}
	}
	for _, path := range also {
		entries = append(entries, entry{path: path, tracked: true})
	}
	st.Worktree = newWorktree(repo.Root, sortedEntries(entries))
	return st, nil
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
			found, head, err := changed(record, kind)
			if err != nil {
				return nil, err
			}
			if kind == "2" && i+1 < len(records) {
				// The object HEAD holds belongs to the original path of a
				// rename, never to the path it was renamed to.
				i++
				found = append(found, entry{path: records[i], tracked: true, head: head})
			}
			entries = append(entries, found...)
		default:
			return nil, fmt.Errorf("%w: git status printed %q", ErrGit, record)
		}
	}
	return sortedEntries(entries), nil
}

// sortedEntries sorts the entries by path and merges the records of one
// path, a deletion from the index beside the untracked file it left
// included, into one entry that keeps what each record knew.
func sortedEntries(entries []entry) []entry {
	slices.SortStableFunc(entries, func(a, b entry) int { return strings.Compare(a.path, b.path) })
	out := entries[:0]
	for _, e := range entries {
		if n := len(out); n > 0 && out[n-1].path == e.path {
			last := &out[n-1]
			last.tracked = last.tracked || e.tracked
			last.dir = last.dir || e.dir
			last.head = cmp.Or(last.head, e.head)
			continue
		}
		out = append(out, e)
	}
	return out
}

// changed reads one record of a tracked path: its letters, its submodule
// field, the object HEAD holds and the path, which may hold spaces, after
// the fixed fields. An ordinary record keeps the object HEAD holds at its
// path; a rename answers it for its original path.
func changed(record, kind string) ([]entry, string, error) {
	fields := map[string]int{"1": ordinaryFields, "2": renamedFields, "u": unmergedFields}[kind]
	parts := strings.SplitN(record, " ", fields)
	if len(parts) != fields || parts[fields-1] == "" {
		return nil, "", fmt.Errorf("%w: git status printed %q", ErrGit, record)
	}
	letters, submodule := parts[1], parts[2]
	if strings.HasPrefix(submodule, "S") && strings.Trim(letters, ".M") == "" {
		return nil, "", nil
	}
	head := ""
	if kind != "u" && strings.Trim(parts[headField], "0") != "" {
		head = parts[headField]
	}
	found := entry{path: parts[fields-1], tracked: true}
	if kind == "1" {
		found.head = head
	}
	return []entry{found}, head, nil
}
