package policyfiles

import (
	"fmt"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

const (
	depsAgeTarget = "deps-age"
	depsAgeCode   = "deps-age-minimum"
	depsAgeDepth  = 8
)

// DepsAgeConfig is policy.deps-age-config: the deps-age target runs the
// kit's depsage with a minimum age of seven days and lets its failure stop
// the target, because a version enters seven days after publication
// (section 3) and the pipeline that runs make ci is the only gate.
func DepsAgeConfig() model.Check {
	return buildCheck{metadata: spec("deps-age-config", ruleSupply, "go"), judge: judgeDepsAge}
}

// depsAgeStep is one station of the delegation walk: a recipe line to
// judge, or the reason of a limit met on the way, which matters only when
// no recipe of the walk proves the floor (D2).
type depsAgeStep struct {
	file   string
	line   int
	text   string
	reason string
}

// depsAgeWalk follows a target through its recipe and its literal
// prerequisites in make's order, each target once and eight levels at
// most, because the floor a target enforces may sit in a target it
// delegates to.
type depsAgeWalk struct {
	m       makefile
	visited map[string]bool
	steps   []depsAgeStep
}

// judgeDepsAge leaves an absent Makefile to policy.makefile, so one defect
// is reported once, and consults the limits that could supply an absent
// target before it names the finding.
func judgeDepsAge(in model.Input) model.Result {
	snapshot := &buildSnapshot{Snapshot: in.Snapshot}
	in.Snapshot = snapshot
	m, err := readMakefile(in, buildMakefile)
	if err != nil {
		return answer(snapshot.failed, err)
	}
	if len(m.files) == 0 {
		return model.Result{Status: model.Measured()}
	}
	scope := depsAgeTarget + " target of " + strings.Join(m.files, ", ")
	rule, found := m.rule(depsAgeTarget)
	if !found {
		if reason := depsAgeAbsent(m, depsAgeTarget); reason != "" {
			return model.Result{Status: model.Unmeasured(reason)}
		}
		message := "add the " + depsAgeTarget + " target to Makefile or an included file, running the kit's depsage with -days 7 as golaw.mk does"
		return depsAgeResult(scope, finding(buildMakefile, 0, depsAgeCode, message))
	}
	walk := depsAgeWalk{m: m, visited: make(map[string]bool)}
	walk.target(depsAgeTarget, 0)
	return walk.verdict(scope, rule)
}

// depsAgeAbsent answers why an absent target cannot be judged: a required
// include the snapshot does not hold could define it, which policy.makefile
// names, or a construct of the reader's limits could supply it; an empty
// reason means the target is definitely absent.
func depsAgeAbsent(m makefile, target string) string {
	for _, include := range m.missing {
		if !include.optional && include.condition == "" {
			return fmt.Sprintf("%s:%d: the required include %s is absent from the snapshot and could define %s; policy.makefile names it",
				include.file, include.line, include.name, target)
		}
	}
	if err := makeTargetLimit(m, target); err != nil {
		return err.Error()
	}
	return ""
}

// target records the recipe of a target, then walks its prerequisites,
// because make runs the prerequisites before the recipe and either may
// carry the check.
func (w *depsAgeWalk) target(name string, depth int) {
	if w.visited[name] {
		return
	}
	w.visited[name] = true
	rule, found := w.m.rule(name)
	if !found {
		if reason := depsAgeAbsent(w.m, name); reason != "" {
			w.steps = append(w.steps, depsAgeStep{reason: reason})
		}
		return
	}
	for _, text := range rule.recipe {
		w.steps = append(w.steps, depsAgeStep{file: rule.file, line: rule.line, text: text})
	}
	for _, prerequisite := range rule.prerequisites {
		w.prerequisite(rule, prerequisite, depth+1)
	}
}

// prerequisite follows a literal prerequisite and names a raw one or a
// delegation deeper than the bound as a limit, because make could resolve
// either to a target that runs the check.
func (w *depsAgeWalk) prerequisite(rule makeRule, name string, depth int) {
	switch {
	case strings.ContainsAny(name, "$%"):
		w.steps = append(w.steps, depsAgeStep{reason: outside(rule.file, rule.line, "the raw prerequisite "+name).Error()})
	case depth > depsAgeDepth:
		w.steps = append(w.steps, depsAgeStep{reason: outside(rule.file, rule.line,
			"the prerequisite "+name+", which delegates deeper than eight levels").Error()})
	default:
		w.target(name, depth)
	}
}

// verdict judges the first recipe of the walk that runs depsage and falls
// back to the first limit met, because an unknown construct matters only
// when nothing else proves the floor; a walk without either runs no check.
func (w *depsAgeWalk) verdict(scope string, rule makeRule) model.Result {
	limit := ""
	for _, step := range w.steps {
		reason := step.reason
		days, invocation := 0, false
		if reason == "" {
			var err error
			days, invocation, err = dependencyAgeCommand(step.file, step.line, step.text)
			if err != nil {
				reason = err.Error()
			}
		}
		if limit == "" {
			limit = reason
		}
		if invocation && days < depsAgeMinimum {
			message := fmt.Sprintf("the %s target runs depsage with an effective minimum age of %d days; pass -days %d or more and let its failure stop the target",
				depsAgeTarget, days, depsAgeMinimum)
			return depsAgeResult(scope, finding(step.file, step.line, depsAgeCode, message))
		}
		if invocation {
			return model.Result{Status: model.Measured(), Scope: scope}
		}
	}
	if limit != "" {
		return model.Result{Status: model.Unmeasured(limit)}
	}
	message := "the " + depsAgeTarget + " target runs no depsage check; run the kit's depsage with -days 7 from it, as golaw.mk does"
	return depsAgeResult(scope, finding(rule.file, rule.line, depsAgeCode, message))
}

// depsAgeResult is a measured result with one finding.
func depsAgeResult(scope string, f model.Finding) model.Result {
	return model.Result{Status: model.Measured(), Scope: scope, Findings: []model.Finding{f}}
}
