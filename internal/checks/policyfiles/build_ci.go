package policyfiles

import (
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

const ciMissingCode = "ci-target-missing"

// CITarget is policy.ci-target: the ci target carries every required check,
// or a scheduled or manually dispatched Actions job runs it, because the
// pipeline is the only gate (section 7, D10); a lefthook.yml is judged only
// when present and only for a command heavier than the binary.
func CITarget() model.Check {
	return buildCheck{metadata: spec("ci-target", ruleLadder, "go"), judge: judgeCITarget}
}

// judgeCITarget leaves an absent Makefile to policy.makefile, so one defect
// is reported once, and keeps the lefthook findings apart from the coverage
// verdict, because an optional rung never limits the pipeline's answer.
func judgeCITarget(in model.Input) model.Result {
	snapshot := &buildSnapshot{Snapshot: in.Snapshot}
	in.Snapshot = snapshot
	m, err := readMakefile(in, buildMakefile)
	if err != nil {
		return answer(snapshot.failed, err)
	}
	if len(m.files) == 0 {
		return model.Result{Status: model.Measured()}
	}
	result, err := ciCoverage(in, m)
	if err != nil {
		return answer(snapshot.failed, err)
	}
	hooks, err := hookCommands(in)
	if err != nil {
		return answer(snapshot.failed, err)
	}
	result.Findings = append(result.Findings, hooks...)
	return result
}

// ciIncludes names an unconditional include the snapshot does not hold,
// which could carry a missing check; policy.makefile owns its finding.
func ciIncludes(m makefile) error {
	for _, include := range m.missing {
		if !include.optional && include.condition == "" {
			return outside(include.file, include.line, "required include "+include.name+" is absent from the snapshot")
		}
	}
	return nil
}

// ciCoverage consults the on-demand evidence and the deferred limits only
// for a target ci does not reach, so a construct the verdict does not
// depend on never leaves the check unmeasured (D2).
func ciCoverage(in model.Input, m makefile) (model.Result, error) {
	if _, found := m.rule("ci"); !found {
		if err := ciIncludes(m); err != nil {
			return model.Result{}, err
		}
		if err := makeTargetLimit(m, "ci"); err != nil {
			return model.Result{}, err
		}
		return model.Result{Status: model.Measured(), Findings: []model.Finding{
			finding(buildMakefile, 0, ciMissingCode, "add ci to Makefile with the required check targets"),
		}}, nil
	}
	reached, graphLimit := ciReachable(m)
	required := RequiredTargets(in.Policy.Capabilities)
	missing := ciMissing(required, reached)
	if len(missing) == 0 {
		return model.Result{Status: model.Measured()}, nil
	}
	demand, demandLimit := demandTargets(in)
	missing = ciMissing(missing, demand)
	if len(missing) != 0 && graphLimit != nil {
		return model.Result{}, ciLimitNaming(graphLimit, missing)
	}
	if len(missing) != 0 && demandLimit != nil {
		return model.Result{}, demandLimit
	}
	rule, _ := m.rule("ci")
	return ciMissingFindings(missing, rule), nil
}

// ciLimitNaming names the targets ci misses beside a stated limit of its
// graph, because the repair is the missing target, not the construct.
func ciLimitNaming(limit error, missing []string) error {
	construct, stated := strings.CutSuffix(limit.Error(), ": "+model.ErrOutsideSubset.Error())
	if !stated {
		return limit
	}
	targets := strings.Join(missing, ", ")
	return fmt.Errorf("%s; ci does not otherwise reach %s; add %s to ci: %w", construct, targets, targets, model.ErrOutsideSubset)
}

func ciMissing(required, reached []string) []string {
	return slices.DeleteFunc(slices.Clone(required), func(target string) bool {
		return slices.Contains(reached, target)
	})
}

func ciMissingFindings(missing []string, rule makeRule) model.Result {
	findings := make([]model.Finding, 0, len(missing))
	for _, target := range missing {
		message := fmt.Sprintf("add %s to ci or run make %s in a scheduled or manually dispatched Actions job", target, target)
		findings = append(findings, finding(rule.file, rule.line, ciMissingCode, message))
	}
	return model.Result{Status: model.Measured(), Findings: findings}
}
