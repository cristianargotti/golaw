package proof

import (
	"fmt"
	"slices"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

// PassCondition names the condition pass.
const PassCondition = "condition"

// The names of the dispatches, as a failure prints them.
const (
	inScan   = "scan"
	inStaged = "staged"
	inCommit = "commit"
	inWrite  = "hook write"
	inRead   = "hook read"
	inShell  = "hook shell"
	inPost   = "hook post"
)

// dispatch is one way the engine is asked to run the registry: a mode and,
// in a hook, the shape of the event.
type dispatch struct {
	name  string
	mode  model.Mode
	shape model.EventShape
}

// dispatches are every mode and hook shape the binary dispatches in.
func dispatches() []dispatch {
	return []dispatch{
		{inScan, model.ModeScan, 0},
		{inStaged, model.ModeStaged, 0},
		{inCommit, model.ModeCommit, 0},
		{inWrite, model.ModeHook, model.ShapeWrite},
		{inRead, model.ModeHook, model.ShapeRead},
		{inShell, model.ModeHook, model.ShapeShell},
		{inPost, model.ModeHook, model.ShapePost},
	}
}

// matrix proves that every registered check declares the condition of its
// family, or its own, and that the engine runs it exactly where that
// condition holds: in every dispatch, under every capability, under none,
// and under every capability but each one it needs.
func matrix(reg *engine.Registry) []Failure {
	var out []Failure
	for _, c := range reg.Checks() {
		spec := c.Spec()
		fail := func(reason string) { out = append(out, Failure{Pass: PassCondition, ID: spec.ID, Reason: reason}) }
		want, ok := ConditionOf(spec.ID)
		switch {
		case !ok:
			fail("no condition: write the scope and the needs of the check in internal/proof/conditions.go")
			continue
		case spec.Scope != want.Scope || !sameSet(spec.Needs, want.Needs):
			fail(fmt.Sprintf("declares scope %s and needs %v, its condition is scope %s and needs %v", spec.Scope, spec.Needs, want.Scope, want.Needs))
			continue
		}
		for _, reason := range applicability(spec, want) {
			fail(reason)
		}
	}
	return out
}

// applicability answers every dispatch and capability set where the engine
// runs the check and its condition says it must not, or the other way.
func applicability(spec model.Spec, want Condition) []string {
	var out []string
	for _, d := range dispatches() {
		for _, set := range capabilitySets(want.Needs) {
			_, runs := engine.Applicability(spec, matrixInput(d, set, spec.Owner))
			held := slices.Contains(runsIn(want.Scope), d.name) && holds(set, want.Needs)
			if runs != held {
				out = append(out, fmt.Sprintf("in %s under %v the engine runs it: %t, its condition says %t", d.name, set, runs, held))
			}
		}
	}
	return out
}

// capabilitySets are every capability, none, and every capability but
// each needed one.
func capabilitySets(needs []string) [][]string {
	sets := make([][]string, 0, 2+len(needs))
	sets = append(sets, model.Capabilities(), []string{})
	for _, need := range needs {
		sets = append(sets, slices.DeleteFunc(model.Capabilities(), func(c string) bool { return c == need }))
	}
	return sets
}

// matrixInput is a dispatch over one target under the proof policy with
// the capability set, and with the owner of the check enrolled as the
// profile when it is not the core.
func matrixInput(d dispatch, capabilities []string, owner string) model.Input {
	p := Policy()
	p.Capabilities = capabilities
	if owner != model.OwnerCore {
		p.Profile = owner
	}
	in := model.Input{Mode: d.mode, Policy: p, Snapshot: NewTree(nil), Targets: []model.File{{Path: "a.go", Class: model.ClassTracked}}}
	if d.mode == model.ModeHook {
		in.Event = &model.Event{Shape: d.shape}
	}
	return in
}

// holds reports whether every needed capability is in the set.
func holds(set, needs []string) bool {
	for _, need := range needs {
		if !slices.Contains(set, need) {
			return false
		}
	}
	return true
}

// sameSet reports whether two capability lists name the same capabilities.
func sameSet(a, b []string) bool {
	x, y := slices.Clone(a), slices.Clone(b)
	slices.Sort(x)
	slices.Sort(y)
	return slices.Equal(x, y)
}
