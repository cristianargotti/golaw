package surface

import (
	"errors"

	"github.com/cristianargotti/golaw/internal/model"
)

// introduced keeps the findings a pre write candidate introduces (M6.6.3):
// a finding whose fingerprint the base file already carries is legacy. The
// fingerprint holds no line, so a moved line is not new; the base findings
// count as a multiset, so a second copy of a legacy call is new. Any other
// input keeps every finding.
func (c check) introduced(in model.Input, f source, found []model.Finding) []model.Finding {
	if !c.certain || len(found) == 0 || in.Mode != model.ModeHook || !writeEvent(in) {
		return found
	}
	snapshot, ok := in.Snapshot.(model.Based)
	if !ok {
		return found
	}
	src, err := snapshot.Base().Read(basePath(in.Event, f.path))
	if errors.Is(err, model.ErrNotInSnapshot) {
		return found
	}
	if err != nil {
		// A base the hook cannot read cannot prove a finding new, and a
		// denial needs that proof.
		return nil
	}
	legacy := map[string]int{}
	base := c.judge(source{path: f.path, src: src, info: describe(f.path, src)}, in)
	for i := range base {
		legacy[c.legacyKey(base[i])]++
	}
	var kept []model.Finding
	for i := range found {
		key := c.legacyKey(found[i])
		if legacy[key] > 0 {
			legacy[key]--
			continue
		}
		kept = append(kept, found[i])
	}
	return kept
}

// basePath is where the base holds the candidate's content: the source
// path of a move, else the path itself.
func basePath(event *model.Event, name string) string {
	for _, c := range event.Candidates {
		if c.Path == name && c.Op == model.CandidateMove {
			return c.From
		}
	}
	return name
}

// legacyKey is what makes a finding the same as one of the base. A check
// whose message carries a measure, a line count, keys by its code, so a
// file already over the limit at the base never denies again and only a
// file that crosses the limit is introduced.
func (c check) legacyKey(f model.Finding) string {
	if c.byCode {
		return f.Code
	}
	return model.Fingerprint(f)
}
