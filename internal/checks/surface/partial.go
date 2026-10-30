package surface

import (
	"fmt"

	"github.com/cristianargotti/golaw/internal/model"
)

// partialView answers a hook candidate the decoder could not reconstruct
// as a file over the read bound: it holds only the added lines of its
// change, which no lexer can read as the file, so the run names it unread
// and leaves it to the post event and the pipeline.
type partialView struct {
	model.Snapshot

	partial map[string]bool
}

func (v partialView) Read(name string) ([]byte, error) {
	if v.partial[name] {
		return nil, fmt.Errorf("%w: %s holds only the added lines of its change", model.ErrTooLarge, name)
	}
	return v.Snapshot.Read(name) //nolint:wrapcheck // the snapshot names the path in its error
}

// Select and Any keep the narrow listing of the snapshot beneath, so a
// hook never lists the whole tree through the view.
func (v partialView) Select(match func(string) bool) []model.File {
	return model.Select(v.Snapshot, match)
}

func (v partialView) Any(match func(string) bool) bool { return model.Any(v.Snapshot, match) }

// withPartial is the input whose snapshot reads the candidates of a hook
// event that are not reconstructed as over the read bound. The introduced
// rule reads the base through the input it was given, not through this
// view.
func withPartial(in model.Input) model.Input {
	if in.Mode != model.ModeHook || in.Event == nil {
		return in
	}
	partial := map[string]bool{}
	for _, c := range in.Event.Candidates {
		if !c.Reconstructed {
			partial[c.Path] = true
		}
	}
	if len(partial) > 0 {
		in.Snapshot = partialView{Snapshot: in.Snapshot, partial: partial}
	}
	return in
}
