package engine

import (
	"fmt"
	"slices"

	"github.com/cristianargotti/golaw/internal/model"
)

// Applicability answers whether a check runs for the input and, when it
// does not, the status Dispatch records in its place: NotApplicable for a
// scope, owner or capability mismatch, Unmeasured for a runtime need the
// mode cannot provide (NotApplicable in hook mode, which is a fast path).
func Applicability(spec model.Spec, in model.Input) (model.Status, bool) {
	if !scopeApplies(spec.Scope, in) {
		return model.NotApplicable(fmt.Sprintf("scope %s does not run in %s", spec.Scope, describe(in))), false
	}
	if spec.Owner != model.OwnerCore && spec.Owner != in.Policy.Profile {
		return model.NotApplicable(fmt.Sprintf("profile %s is not enrolled", spec.Owner)), false
	}
	for _, need := range spec.Needs {
		if !in.Policy.Has(need) {
			return model.NotApplicable(fmt.Sprintf("capability %s is not enabled", need)), false
		}
	}
	available := provided(in.Mode)
	for _, need := range spec.Requires {
		if slices.Contains(available, need) {
			continue
		}
		reason := fmt.Sprintf("%s is not available in %s mode", need, in.Mode)
		if in.Mode == model.ModeHook {
			return model.NotApplicable(reason), false
		}
		return model.Unmeasured(reason), false
	}
	return model.Measured(), true
}

// scopeApplies is the table of section 5.2 as the engine reads it: file and
// tree checks run over a scan, file checks also run over a hook write or
// post, event checks run in every hook shape, commit checks only in the
// commit rung.
func scopeApplies(scope model.Scope, in model.Input) bool {
	switch in.Mode {
	case model.ModeScan, model.ModeStaged:
		return scope == model.ScopeFile || scope == model.ScopeTree
	case model.ModeCommit:
		return scope == model.ScopeCommit
	case model.ModeHook:
		return hookScopeApplies(scope, in.Event.Shape)
	default:
		return false
	}
}

func hookScopeApplies(scope model.Scope, shape model.EventShape) bool {
	switch scope {
	case model.ScopeEvent:
		return true
	case model.ScopeFile:
		return shape == model.ShapeWrite || shape == model.ShapePost
	case model.ScopeTree, model.ScopeCommit:
		return false
	default:
		return false
	}
}

// provided lists the runtime needs a mode can satisfy. External evidence
// is never provided in this version, so a check that requires it stays
// Unmeasured until the profile loader of a later milestone supplies it.
func provided(mode model.Mode) []string {
	if mode == model.ModeScan || mode == model.ModeStaged {
		return []string{"git-history", "go-types"}
	}
	if mode == model.ModeCommit {
		return []string{"git-history"}
	}
	return nil
}

func describe(in model.Input) string {
	if in.Mode == model.ModeHook && in.Event != nil {
		return in.Mode.String() + " " + in.Event.Shape.String()
	}
	return in.Mode.String() + " mode"
}
