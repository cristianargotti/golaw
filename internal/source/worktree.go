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
// universe git lists, every path classified, the bytes of each readable
// file within the bound held in memory, and one identity over all of it.
type Worktree struct {
	root     string
	files    []model.File
	index    map[string]int
	content  map[string][]byte
	identity string
}

// NewWorktree builds the snapshot of repo. Every regular file within
// model.MaxReadBytes is read once, now, and nothing is read later, so the
// snapshot answers the same bytes whatever happens on disk afterwards.
func NewWorktree(ctx context.Context, repo Repo) (*Worktree, error) {
	entries, err := listUniverse(ctx, repo.Root)
	if err != nil {
		return nil, err
	}
	w := &Worktree{
		root:    repo.Root,
		files:   make([]model.File, 0, len(entries)),
		index:   make(map[string]int, len(entries)),
		content: make(map[string][]byte),
	}
	var ledger []byte
	linked := map[string]bool{}
	for _, e := range entries {
		file, content, read := inspect(repo.Root, e, linked)
		if read {
			w.content[file.Path] = content
		}
		w.index[file.Path] = len(w.files)
		w.files = append(w.files, file)
		ledger = append(ledger, file.Path+"\x00"+file.Class.String()+"\x00"+ContentHash(content)+"\n"...)
	}
	w.identity = ContentHash(ledger)
	return w, nil
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
// universe: the snapshot identity a report prints.
func (w *Worktree) Identity() string { return w.identity }

// Files lists the universe in path order; the slice is a copy.
func (w *Worktree) Files() []model.File { return slices.Clone(w.files) }

// Read returns a copy of the frozen bytes of a path. A path outside the
// universe is ErrNotInSnapshot, a file beyond the bound is ErrTooLarge and
// every class that holds no bytes (symlink, gitlink, unreadable) is
// ErrUnreadable; a partial file is never returned.
func (w *Worktree) Read(rel string) ([]byte, error) {
	i, ok := w.index[rel]
	if !ok {
		return nil, fmt.Errorf("%w: %s", model.ErrNotInSnapshot, rel)
	}
	if content, read := w.content[rel]; read {
		return bytes.Clone(content), nil
	}
	file := w.files[i]
	if file.Class.IsText() && file.Size > model.MaxReadBytes {
		return nil, fmt.Errorf("%w: %s is %d bytes", model.ErrTooLarge, rel, file.Size)
	}
	return nil, fmt.Errorf("%w: %s is %s", model.ErrUnreadable, rel, file.Class)
}

// Class answers the class of a path, ClassUnknown outside the universe.
func (w *Worktree) Class(rel string) model.FileClass {
	if i, ok := w.index[rel]; ok {
		return w.files[i].Class
	}
	return model.ClassUnknown
}
