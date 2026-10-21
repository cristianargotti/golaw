package goast

import (
	"fmt"

	"github.com/cristianargotti/golaw/internal/model"
)

// partialView answers a hook candidate the decoder could not reconstruct
// as a file over the read bound: it holds only the added lines of its
// change, which no rule of the family can parse, and every rule already
// leaves such a file to the post event and the pipeline, never to a
// finding.
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
// directory rule of a hook still asks for one directory only.
func (v partialView) Select(match func(string) bool) []model.File {
	return model.Select(v.Snapshot, match)
}

func (v partialView) Any(match func(string) bool) bool { return model.Any(v.Snapshot, match) }

// withPartial is the input whose snapshot reads the candidates of a hook
// event that are not reconstructed as over the read bound.
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
