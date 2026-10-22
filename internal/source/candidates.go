package source

import "github.com/cristianargotti/golaw/internal/model"

// FromCandidates maps the candidates a hook decoder reconstructed, with
// their bytes, to the mutations the overlay folds. An edit or a move the
// decoder applied arrives as its whole content, folded as a deletion of
// the source of a move and a write of the content bound to the same base;
// one it could not apply arrives as its added lines, folded so that only
// those are judged and the candidate stays not reconstructed.
func FromCandidates(candidates []model.Candidate, content map[string][]byte) []Mutation {
	muts := make([]Mutation, 0, len(candidates))
	for _, c := range candidates {
		bytes, held := content[c.Path]
		m := Mutation{Path: c.Path, Op: c.Op, From: c.From, BaseHash: c.BaseHash, Content: bytes}
		switch {
		case c.Op == model.CandidateEdit && c.Reconstructed:
			m.Op = model.CandidateReplace
		case (c.Op == model.CandidateEdit || c.Op == model.CandidateMove) && !c.Reconstructed:
			m.Content, m.Edits = nil, []Edit{{New: string(bytes)}}
		case c.Op == model.CandidateMove && held:
			muts = append(muts, Mutation{Path: c.From, Op: model.CandidateDelete, BaseHash: c.BaseHash})
			m = Mutation{Path: c.Path, Op: model.CandidateReplace, Content: bytes}
		}
		muts = append(muts, m)
	}
	return muts
}
