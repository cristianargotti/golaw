package proof

import (
	"context"
	"fmt"
	"slices"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

// PassCapability names the capability pass.
const PassCapability = "capability"

// Capability proves that a check conditioned by a capability applies only
// where the capability holds: for every entry whose registered check
// declares Needs, the red fixture under the policy of the entry without each needed
// capability answers NotApplicable for the check and yields no finding of
// the entry's code from any check.
func Capability(ctx context.Context, reg *engine.Registry, root string, m Manifest) []Failure {
	return eachEntry(reg, root, m, PassCapability, func(p pair) []Failure { return p.capability(ctx, reg) })
}

// capability runs the red fixture once per needed capability, that one off.
func (p pair) capability(ctx context.Context, reg *engine.Registry) []Failure {
	var out []Failure
	for _, need := range p.spec.Needs {
		if reason := p.without(ctx, reg, need); reason != "" {
			out = append(out, Failure{Pass: PassCapability, ID: p.entry.ID, Reason: reason})
		}
	}
	return out
}

// without answers why the red fixture breaks the pass with one capability
// off, empty when the pass holds for it.
func (p pair) without(ctx context.Context, reg *engine.Registry, need string) string {
	policy := *p.policy
	policy.Capabilities = slices.DeleteFunc(slices.Clone(policy.Capabilities), func(c string) bool { return c == need })
	v, err := p.runUnder(ctx, reg, p.red, &policy)
	switch {
	case err != nil:
		return "red fixture: " + err.Error()
	case v.status.Kind != model.StatusNotApplicable:
		return fmt.Sprintf("with capability %s off the check was %s: it runs where its capability does not hold", need, v.status.Kind)
	case v.other != "":
		return fmt.Sprintf("with capability %s off %s still reports %s: the red case is judged where the capability does not hold", need, v.other, p.entry.Code)
	}
	return ""
}

// runUnder dispatches one fixture of the entry under a policy and answers
// what the run says about the entry's check.
func (p pair) runUnder(ctx context.Context, reg *engine.Registry, f Fixture, policy *model.EffectivePolicy) (verdict, error) {
	in, err := f.Input(p.spec.Scope, policy)
	if err != nil {
		return verdict{}, err
	}
	run, err := engine.Dispatch(ctx, reg, in)
	if err != nil {
		return verdict{}, fmt.Errorf("dispatch: %w", err)
	}
	var v verdict
	for i := range run.Outcomes {
		o := &run.Outcomes[i]
		if o.Spec.ID == p.entry.ID {
			v.status = o.Result.Status
		}
		for k := range o.Result.Findings {
			f := &o.Result.Findings[k]
			if f.Code != p.entry.Code {
				continue
			}
			if o.Spec.ID == p.entry.ID {
				v.own = true
			} else if v.other == "" {
				v.other = o.Spec.ID
			}
		}
	}
	return v, nil
}
