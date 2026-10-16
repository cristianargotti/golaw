package hook

import (
	"fmt"

	"github.com/cristianargotti/golaw/internal/model"
)

// deferredSnapshot asks for the worktree the first time the decoder or a
// rule reads it, so an event whose rules never read the tree, a Read among
// them, lists none. A listing that fails answers an empty universe and is
// kept, and the verdict it could have changed becomes cannot judge.
type deferredSnapshot struct {
	load     func() (model.Snapshot, error)
	snapshot model.Snapshot
	err      error
	asked    bool
}

// noTree is the universe of a listing that failed: it holds nothing.
type noTree struct{ err error }

func (noTree) Files() []model.File                   { return nil }
func (t noTree) Read(string) ([]byte, error)         { return nil, t.err }
func (noTree) Class(string) model.FileClass          { return model.ClassUnknown }
func (noTree) Select(func(string) bool) []model.File { return nil }
func (noTree) Any(func(string) bool) bool            { return false }

func deferSnapshot(load func() (model.Snapshot, error)) *deferredSnapshot {
	return &deferredSnapshot{load: load}
}

func (d *deferredSnapshot) Files() []model.File { return d.get().Files() }

func (d *deferredSnapshot) Read(path string) ([]byte, error) {
	return d.get().Read(path) //nolint:wrapcheck // the snapshot's own error is the reason a rule reports
}

func (d *deferredSnapshot) Class(path string) model.FileClass { return d.get().Class(path) }

func (d *deferredSnapshot) Select(match func(string) bool) []model.File {
	return model.Select(d.get(), match)
}

func (d *deferredSnapshot) Any(match func(string) bool) bool { return model.Any(d.get(), match) }

func (d *deferredSnapshot) get() model.Snapshot {
	if !d.asked {
		d.asked = true
		d.snapshot, d.err = d.load()
		if d.err == nil && d.snapshot == nil {
			d.err = fmt.Errorf("%w: the worktree is absent", model.ErrCannotJudge)
		}
		if d.err != nil {
			d.snapshot = noTree{err: d.err}
		}
	}
	return d.snapshot
}

// failure answers why the worktree could not be read, nil when it was read
// or never asked for.
func (d *deferredSnapshot) failure() error {
	if d.err == nil {
		return nil
	}
	return fmt.Errorf("%w: snapshot: %w", model.ErrCannotJudge, d.err)
}
