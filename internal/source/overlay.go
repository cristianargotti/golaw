package source

import (
	"bytes"
	"fmt"
	"slices"

	"github.com/cristianargotti/golaw/internal/model"
)

// Edit is one replacement a write event applies to the content on disk,
// in the form the agents express it: the old text, the new text and
// whether every occurrence is replaced.
type Edit struct {
	Old string
	New string
	All bool
}

// Mutation is one candidate change a hook decoded from a write event,
// before the overlay binds it to the snapshot. Content carries the whole
// new content of a create or a replace; Edits carry the edits of an edit
// and of a move (applied to the moved content); From names the source of
// a move. BaseHash, when set, is the content hash the agent's view was
// computed on and must match the snapshot.
type Mutation struct {
	Path     string
	Op       model.CandidateOp
	From     string
	Content  []byte
	Edits    []Edit
	BaseHash string
}

// Overlay is a snapshot with the candidates of a write event folded in: a
// check reads a candidate exactly as it reads a committed file, and a
// deleted or moved path is gone from the universe.
type Overlay struct {
	base    model.Snapshot
	files   map[string]model.File
	content map[string][]byte
	moved   map[string]string // a destination without bytes to its base source
	removed map[string]bool
	all     []model.File
	listed  bool
}

// NewOverlay folds the mutations into base, in order, and returns the
// overlay with the candidates the event carries: the operation settled
// against the base (a whole-content write of an absent path is a create,
// of a present path a replace), Reconstructed true when the whole content
// is known and BaseHash the hash of the content an edit, a replace, a
// deletion or a move started from. A path that escapes the root is
// ErrEscapedRoot; a path that is or traverses a symbolic link of the base
// is ErrUnsafeLink; a bound hash that differs from the base is
// ErrBaseMismatch; everything else the overlay cannot apply is ErrOverlay.
func NewOverlay(base model.Snapshot, muts []Mutation) (*Overlay, []model.Candidate, error) {
	o := &Overlay{
		base:    base,
		files:   map[string]model.File{},
		content: map[string][]byte{},
		moved:   map[string]string{},
		removed: map[string]bool{},
	}
	candidates := make([]model.Candidate, 0, len(muts))
	for _, m := range muts {
		c, err := o.apply(m)
		if err != nil {
			return nil, nil, err
		}
		candidates = append(candidates, c)
	}
	return o, candidates, nil
}

// Files lists the base universe with the candidates folded in, in path
// order; the slice is a copy.
func (o *Overlay) Files() []model.File {
	if !o.listed {
		o.all, o.listed = o.fold(o.base.Files(), func(string) bool { return true }), true
	}
	return slices.Clone(o.all)
}

// Select lists the files whose paths match with the candidates folded in,
// in path order, asking the base for the matching paths only.
func (o *Overlay) Select(match func(path string) bool) []model.File {
	return o.fold(model.Select(o.base, match), match)
}

// Any reports whether a path of the overlay universe matches, without
// reading a file.
func (o *Overlay) Any(match func(path string) bool) bool {
	for rel := range o.files {
		if match(rel) {
			return true
		}
	}
	return model.Any(o.base, func(rel string) bool { return match(rel) && !o.removed[rel] })
}

// Read answers the candidate bytes for a candidate path, the base bytes
// for any other path, and ErrNotInSnapshot for a deleted or moved one. A
// candidate beyond model.MaxReadBytes is ErrTooLarge, as a file on disk
// is, so a check never reads a write the snapshot would refuse.
func (o *Overlay) Read(rel string) ([]byte, error) {
	if o.removed[rel] {
		return nil, fmt.Errorf("%w: %s", model.ErrNotInSnapshot, rel)
	}
	if content, ok := o.content[rel]; ok {
		if len(content) > model.MaxReadBytes {
			return nil, fmt.Errorf("%w: %s is %d bytes", model.ErrTooLarge, rel, len(content))
		}
		return bytes.Clone(content), nil
	}
	if from, ok := o.moved[rel]; ok {
		rel = from
	}
	data, err := o.base.Read(rel)
	if err != nil {
		return nil, fmt.Errorf("overlay base: %w", err)
	}
	return data, nil
}

// Class answers the candidate class for a candidate path, the base class
// otherwise and ClassUnknown for a deleted or moved path.
func (o *Overlay) Class(rel string) model.FileClass {
	if o.removed[rel] {
		return model.ClassUnknown
	}
	if f, ok := o.files[rel]; ok {
		return f.Class
	}
	return o.base.Class(rel)
}

// apply folds one mutation and returns its candidate.
func (o *Overlay) apply(m Mutation) (model.Candidate, error) {
	rel, err := o.target(m.Path)
	if err != nil {
		return model.Candidate{}, err
	}
	switch m.Op {
	case model.CandidateCreate, model.CandidateReplace:
		return o.write(rel, m)
	case model.CandidateEdit:
		return o.edit(rel, m)
	case model.CandidateDelete:
		return o.remove(rel, m)
	case model.CandidateMove:
		return o.move(rel, m)
	default:
		return model.Candidate{}, fmt.Errorf("%w: %s has operation %s", ErrOverlay, m.Path, m.Op)
	}
}

// target validates a candidate path and refuses one that is or crosses a
// symbolic link of the base, because a write there lands outside the path
// the agent named.
func (o *Overlay) target(given string) (string, error) {
	rel, err := CleanRel(given)
	if err != nil {
		return "", err
	}
	for probe := rel; probe != "."; probe = parentOf(probe) {
		if o.base.Class(probe) == model.ClassSymlink {
			return "", fmt.Errorf("%w: %s through %s", ErrUnsafeLink, rel, probe)
		}
	}
	return rel, nil
}

// write folds a whole-content write: a create of an absent path or a
// replace of a present one, bound to the base content when there is one.
// The content is copied: the caller keeps its slice, and a frozen
// snapshot must not change under a write the caller makes later.
func (o *Overlay) write(rel string, m Mutation) (model.Candidate, error) {
	c := model.Candidate{Path: rel, Op: model.CandidateCreate, Reconstructed: true}
	if o.present(rel) {
		c.Op = model.CandidateReplace
		hash, err := o.bind(rel, m.BaseHash)
		if err != nil {
			return model.Candidate{}, err
		}
		c.BaseHash = hash
	}
	o.put(rel, bytes.Clone(m.Content), o.tracked(rel))
	return c, nil
}

// edit folds an edit of a text path: the edits are applied to the current
// content, and when one cannot be applied only the added text is judged.
func (o *Overlay) edit(rel string, m Mutation) (model.Candidate, error) {
	if len(m.Edits) == 0 {
		return model.Candidate{}, fmt.Errorf("%w: edit of %s carries no edit", ErrOverlay, rel)
	}
	current, err := o.text(rel)
	if err != nil {
		return model.Candidate{}, err
	}
	hash, err := o.bind(rel, m.BaseHash)
	if err != nil {
		return model.Candidate{}, err
	}
	c := model.Candidate{Path: rel, Op: model.CandidateEdit, BaseHash: hash}
	next, applied := applyEdits(current, m.Edits)
	c.Reconstructed = applied
	if !applied {
		next = addedText(m.Edits)
	}
	o.put(rel, next, o.tracked(rel))
	return c, nil
}

// remove folds a deletion of a present path, bound to the content it
// removes, so a deletion decided on bytes the agent never saw is refused.
func (o *Overlay) remove(rel string, m Mutation) (model.Candidate, error) {
	if !o.present(rel) {
		return model.Candidate{}, fmt.Errorf("%w: delete of absent %s", ErrOverlay, rel)
	}
	hash, err := o.bind(rel, m.BaseHash)
	if err != nil {
		return model.Candidate{}, err
	}
	o.drop(rel)
	return model.Candidate{Path: rel, Op: model.CandidateDelete, Reconstructed: true, BaseHash: hash}, nil
}

// move folds a move: the destination takes the source content, with the
// edits applied when there are any, and the source leaves the universe;
// the source content is bound for the same reason as a deletion's.
func (o *Overlay) move(rel string, m Mutation) (model.Candidate, error) {
	from, err := o.target(m.From)
	if err != nil {
		return model.Candidate{}, err
	}
	if !o.present(from) {
		return model.Candidate{}, fmt.Errorf("%w: move of absent %s", ErrOverlay, from)
	}
	hash, err := o.bind(from, m.BaseHash)
	if err != nil {
		return model.Candidate{}, err
	}
	c := model.Candidate{Path: rel, Op: model.CandidateMove, From: from, Reconstructed: true, BaseHash: hash}
	content, readable := o.bytes(from)
	tracked := o.tracked(from)
	if len(m.Edits) > 0 && readable {
		if next, applied := applyEdits(content, m.Edits); applied {
			content = next
		} else {
			content, c.Reconstructed = addedText(m.Edits), false
		}
	}
	o.drop(from)
	if readable {
		o.put(rel, content, tracked)
		return c, nil
	}
	o.moved[rel] = from
	o.files[rel] = model.File{Path: rel, Class: o.base.Class(from), Size: o.baseSize(from)}
	return c, nil
}
