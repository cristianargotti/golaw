package policy

import (
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// CheckoutLadder is the ladder of the golaw.yml the checkout holds, read
// from the file as it stands, so a person switches a rung off without a
// commit (decision 10: the ladder is local convenience, never a floor). It
// answers false when the checkout holds no regular golaw.yml within
// model.MaxPolicyBytes or the file does not decode; the caller then uses
// the ladder of the policy in force.
func CheckoutLadder(root string) (model.Ladder, bool) {
	data, ok := checkoutPolicy(root)
	if !ok {
		return model.Ladder{}, false
	}
	f, err := Decode(data)
	if err != nil {
		return model.Ladder{}, false
	}
	return f.Ladder.ladder(), true
}

// ladder answers the rungs a ladder block arms, the default of
// model.DefaultLadder for every key the block leaves unset.
func (l *Ladder) ladder() model.Ladder {
	out := model.DefaultLadder()
	if l == nil {
		return out
	}
	set := func(field, value *bool) {
		if value != nil {
			*field = *value
		}
	}
	set(&out.PreCommit, l.PreCommit)
	set(&out.CommitMsg, l.CommitMsg)
	set(&out.PrePush, l.PrePush)
	set(&out.TestStaged, l.TestStaged)
	set(&out.SurfaceLint, l.SurfaceLint)
	set(&out.HookFeedback, l.HookFeedback)
	return out
}

// waivers answers every committed waiver sorted by advisory id, the
// expired ones included, because the push rung names an expired waiver in
// its refusal instead of reporting the advisory as never waived.
func (f File) waivers() []model.Waiver {
	out := make([]model.Waiver, 0, len(f.Waivers))
	for _, w := range f.Waivers {
		out = append(out, model.Waiver{ID: w.ID, Until: w.Until, Record: w.Record})
	}
	slices.SortFunc(out, func(a, b model.Waiver) int { return strings.Compare(a.ID, b.ID) })
	return out
}
