package source

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"

	"github.com/cristianargotti/golaw/internal/model"
)

// Worktree is the frozen snapshot of a repository's working tree: the
// universe git lists, each path inspected and classified the first time a
// caller asks for it, the bytes of each readable file within the bound
// held in memory from that first read on, and one identity over all of it.
type Worktree struct {
	root     string
	entries  []entry
	index    map[string]int
	seen     map[string]inspection
	dirs     map[string]int
	files    []model.File
	listed   bool
	identity string
}

// ledgerLine is the room one path takes in the identity ledger, its path
// aside: two separators, a class name, a content hash and a newline.
const ledgerLine = 96

// inspection is what one path answered the first time it was asked for.
type inspection struct {
	file    model.File
	content []byte
	stamp   string
	read    bool
}

// NewWorktree lists the universe of repo now and reads nothing yet. Each
// path is inspected once, when a caller first asks for it, and answers the
// same class and bytes on every later call whatever happens on disk
// afterwards, so a hook reads only the files its rules ask for and a scan
// that asks for every file reads each once.
func NewWorktree(ctx context.Context, repo Repo) (*Worktree, error) {
	entries, err := listUniverse(ctx, repo.Root)
	if err != nil {
		return nil, err
	}
	return newWorktree(repo.Root, entries), nil
}

// newWorktree is the snapshot of the listed entries under a root, nothing
// inspected yet.
func newWorktree(root string, entries []entry) *Worktree {
	index := make(map[string]int, len(entries))
	for i, e := range entries {
		index[e.path] = i
	}
	return &Worktree{root: root, entries: entries, index: index, seen: map[string]inspection{}, dirs: map[string]int{}}
}

// Refreshed answers a snapshot of the same listing that has inspected
// nothing yet, so each path is read from disk again: what changed since
// this snapshot first read a path shows, and a path the listing left out
// stays out.
func (w *Worktree) Refreshed() *Worktree {
	return &Worktree{root: w.root, entries: w.entries, index: w.index, seen: map[string]inspection{}, dirs: map[string]int{}}
}

// ContentHash is the lower case hex sha256 of content: the content
// identity a candidate is bound to and the unit of the snapshot identity.
func ContentHash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// Root is the real path of the worktree root.
func (w *Worktree) Root() string { return w.root }

// Identity is the sha256 over every path, class and content hash of the
// universe: the snapshot identity a report prints. It reads every file.
func (w *Worktree) Identity() string {
	if w.identity == "" {
		files := w.Files()
		ledger := make([]byte, 0, len(files)*ledgerLine)
		for _, file := range files {
			got, _ := w.inspected(file.Path)
			ledger = append(ledger, file.Path+"\x00"+file.Class.String()+"\x00"+ContentHash(got.content)+"\n"...)
		}
		w.identity = ContentHash(ledger)
	}
	return w.identity
}

// Files lists the universe in path order, every path inspected; the slice
// is a copy.
func (w *Worktree) Files() []model.File {
	if !w.listed {
		w.files, w.listed = w.Select(func(string) bool { return true }), true
	}
	return slices.Clone(w.files)
}

// Select lists the files of the universe whose paths match, in path order,
// inspecting only those.
func (w *Worktree) Select(match func(path string) bool) []model.File {
	var out []model.File
	for _, e := range w.entries {
		if !match(e.path) {
			continue
		}
		if got, present := w.inspected(e.path); present {
			out = append(out, got.file)
		}
	}
	return out
}

// Any reports whether a path of the universe matches without reading a
// file: a listed path is present unless it or a directory above it is gone.
func (w *Worktree) Any(match func(path string) bool) bool {
	for _, e := range w.entries {
		if !match(e.path) {
			continue
		}
		if got, done := w.seen[e.path]; done && got.file.Class != model.ClassUnknown || !done && present(w.root, e, w.dirs) {
			return true
		}
	}
	return false
}

// Read returns a copy of the frozen bytes of a path. A path outside the
// universe is ErrNotInSnapshot, a file beyond the bound is ErrTooLarge and
// every class that holds no bytes (symlink, gitlink, unreadable) is
// ErrUnreadable; a partial file is never returned.
func (w *Worktree) Read(rel string) ([]byte, error) {
	got, present := w.inspected(rel)
	if !present {
		return nil, fmt.Errorf("%w: %s", model.ErrNotInSnapshot, rel)
	}
	if got.read {
		return bytes.Clone(got.content), nil
	}
	if got.file.Class.IsText() && got.file.Size > model.MaxReadBytes {
		return nil, fmt.Errorf("%w: %s is %d bytes", model.ErrTooLarge, rel, got.file.Size)
	}
	return nil, fmt.Errorf("%w: %s is %s", model.ErrUnreadable, rel, got.file.Class)
}

// Class answers the class of a path, ClassUnknown outside the universe.
func (w *Worktree) Class(rel string) model.FileClass {
	got, _ := w.inspected(rel)
	return got.file.Class
}

// StatIdentity answers the cached lstat stamp of a regular file originally
// over the read bound, or nothing when no such stat was held.
func (w *Worktree) StatIdentity(rel string) string {
	got, _ := w.inspected(rel)
	return got.stamp
}

// inspected answers the inspection of a listed path, made the first time
// it is asked for, and whether the path is in the universe: a listed path
// that is gone answers ClassUnknown and is not.
func (w *Worktree) inspected(rel string) (inspection, bool) {
	i, listed := w.index[rel]
	if !listed {
		return inspection{}, false
	}
	got, done := w.seen[rel]
	if !done {
		got = inspect(w.root, w.entries[i], w.dirs)
		w.seen[rel] = got
	}
	return got, got.file.Class != model.ClassUnknown
}
