package source

import (
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// present reports whether a path is in the overlay universe.
func (o *Overlay) present(rel string) bool {
	if o.removed[rel] {
		return false
	}
	_, folded := o.files[rel]
	return folded || o.base.Class(rel) != model.ClassUnknown
}

// tracked reports whether the current entry of a path is one git tracks,
// so a replaced tracked file stays tracked and a new path is untracked.
func (o *Overlay) tracked(rel string) bool {
	return o.present(rel) && o.Class(rel) == model.ClassTracked
}

// bytes answers the current content of a present path and whether it
// holds readable bytes.
func (o *Overlay) bytes(rel string) ([]byte, bool) {
	if content, ok := o.content[rel]; ok {
		return content, true
	}
	data, err := o.base.Read(rel)
	return data, err == nil
}

// text answers the current content of a path that must hold text.
func (o *Overlay) text(rel string) ([]byte, error) {
	if !o.present(rel) {
		return nil, fmt.Errorf("%w: edit of absent %s", ErrOverlay, rel)
	}
	if !o.Class(rel).IsText() {
		return nil, fmt.Errorf("%w: edit of %s, which is %s", ErrOverlay, rel, o.Class(rel))
	}
	content, readable := o.bytes(rel)
	if !readable {
		_, err := o.base.Read(rel)
		return nil, fmt.Errorf("%w: edit of %s: %w", ErrOverlay, rel, err)
	}
	return content, nil
}

// bind answers the hash of the current content of a path and refuses a
// bound hash that differs from it, so a verdict is never computed on bytes
// the agent did not see.
func (o *Overlay) bind(rel, bound string) (string, error) {
	content, readable := o.bytes(rel)
	hash := ""
	if readable {
		hash = ContentHash(content)
	}
	if bound != "" && bound != hash {
		return "", fmt.Errorf("%w: %s", ErrBaseMismatch, rel)
	}
	return hash, nil
}

// put lifts a deletion or a move of the path first, so a candidate that
// writes a path an earlier candidate removed brings it back with the class
// of its new bytes.
func (o *Overlay) put(rel string, content []byte, tracked bool) {
	delete(o.removed, rel)
	delete(o.moved, rel)
	o.content[rel] = content
	o.files[rel] = model.File{Path: rel, Class: Classify(rel, content, tracked), Size: int64(len(content))}
}

// drop marks the path removed rather than only forgetting its candidate,
// because the base still holds the path and would show through.
func (o *Overlay) drop(rel string) {
	delete(o.files, rel)
	delete(o.content, rel)
	delete(o.moved, rel)
	o.removed[rel] = true
}

// baseSize answers the size the base knows for a path, 0 when absent.
func (o *Overlay) baseSize(rel string) int64 {
	for _, f := range model.Select(o.base, func(path string) bool { return path == rel }) {
		return f.Size
	}
	return 0
}

// fold lays the candidates over files of the base, the ones that match:
// a removed path leaves, a folded path takes the place of its base entry
// and a new path joins, all in path order.
func (o *Overlay) fold(base []model.File, match func(string) bool) []model.File {
	out := make([]model.File, 0, len(base)+len(o.files))
	inBase := make(map[string]bool, len(base))
	for _, f := range base {
		inBase[f.Path] = true
		if folded, ok := o.files[f.Path]; ok {
			out = append(out, folded)
		} else if !o.removed[f.Path] {
			out = append(out, f)
		}
	}
	for rel, f := range o.files {
		if !inBase[rel] && match(rel) {
			out = append(out, f)
		}
	}
	slices.SortFunc(out, func(a, b model.File) int { return strings.Compare(a.Path, b.Path) })
	return out
}

// parentOf is the directory of a slash separated path, "." at the top.
func parentOf(rel string) string {
	if dir, _, found := strings.CutLast(rel, "/"); found {
		return dir
	}
	return "."
}

// applyEdits applies the edits in order the way the agents do: an edit
// with All replaces every occurrence of a present old text; otherwise the
// old text must occur exactly once. It reports false when an edit cannot
// be applied, and the caller then judges only the added text.
func applyEdits(content []byte, edits []Edit) ([]byte, bool) {
	text := string(content)
	for _, e := range edits {
		count := strings.Count(text, e.Old)
		switch {
		case e.Old == "" || count == 0:
			return nil, false
		case e.All:
			text = strings.ReplaceAll(text, e.Old, e.New)
		case count != 1:
			return nil, false
		default:
			text = strings.Replace(text, e.Old, e.New, 1)
		}
	}
	return []byte(text), true
}

// addedText is what a check judges when a candidate could not be
// reconstructed: the new text of every edit, one per line.
func addedText(edits []Edit) []byte {
	parts := make([]string, 0, len(edits))
	for _, e := range edits {
		parts = append(parts, e.New)
	}
	return []byte(strings.Join(parts, "\n"))
}
