package gen

import (
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// pending is the snapshot a kind reads during a run: the bytes the run read,
// with the writes still pending laid over them, so a kind rendered after
// another one sees the file the first one creates and appends to it instead
// of rendering it whole a second time.
type pending struct {
	base   model.Snapshot
	writes map[string]Write
}

// overlay answers the snapshot of the run so far; with no pending write it
// is the snapshot itself, so a run that writes nothing reads as before.
func (p *plan) overlay() model.Snapshot {
	if len(p.writes) == 0 {
		return p.in.Snapshot
	}
	return pending{base: p.in.Snapshot, writes: p.writes}
}

func (s pending) Read(path string) ([]byte, error) {
	if w, ok := s.writes[path]; ok {
		if w.Mode == WriteDelete {
			return nil, model.ErrNotInSnapshot
		}
		return w.Content, nil
	}
	content, err := s.base.Read(path)
	if err != nil {
		return nil, err //nolint:wrapcheck // the snapshot's own sentinel, which callers compare with errors.Is
	}
	return content, nil
}

func (s pending) Class(path string) model.FileClass {
	if w, ok := s.writes[path]; ok && w.Mode == WriteDelete {
		return model.ClassUnknown
	}
	if class := s.base.Class(path); class != model.ClassUnknown {
		return class
	}
	if _, ok := s.writes[path]; ok {
		return model.ClassUntracked
	}
	return model.ClassUnknown
}

func (s pending) Files() []model.File {
	return s.merge(s.base.Files(), func(string) bool { return true })
}

// Select keeps the base's own selection, so a hook snapshot is never walked
// whole because a run has pending writes.
func (s pending) Select(match func(path string) bool) []model.File {
	return s.merge(model.Select(s.base, match), match)
}

func (s pending) Any(match func(path string) bool) bool {
	if model.Any(s.base, match) {
		return true
	}
	for path, w := range s.writes {
		if w.Mode != WriteDelete && match(path) {
			return true
		}
	}
	return false
}

// merge adds the pending creations that match to files, in path order.
func (s pending) merge(files []model.File, match func(path string) bool) []model.File {
	out := slices.DeleteFunc(slices.Clone(files), func(f model.File) bool {
		w, ok := s.writes[f.Path]
		return ok && w.Mode == WriteDelete
	})
	for path, w := range s.writes {
		if w.Mode != WriteDelete && match(path) && s.base.Class(path) == model.ClassUnknown {
			out = append(out, model.File{Path: path, Class: model.ClassUntracked, Size: int64(len(w.Content))})
		}
	}
	slices.SortFunc(out, func(a, b model.File) int { return strings.Compare(a.Path, b.Path) })
	return out
}
