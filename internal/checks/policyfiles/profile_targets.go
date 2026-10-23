package policyfiles

import (
	"fmt"

	"github.com/cristianargotti/golaw/internal/model"
)

// ProfileTargets is policy.profile-targets: every make target the profile
// adds is a prerequisite of ci, directly or through a target of the file,
// or a scheduled or manually dispatched Actions job runs it (D15). The
// core table stays policy.ci-target's; the profile only adds to it.
func ProfileTargets() model.Check {
	return buildCheck{metadata: spec("profile-targets", ruleProfile, "go"), judge: judgeProfileTargets}
}

// judgeProfileTargets leaves an absent Makefile to policy.makefile and an
// absent ci target to policy.ci-target, so one defect is reported once.
func judgeProfileTargets(in model.Input) model.Result {
	if status, ok := section(in, model.SectionTargets); !ok {
		return model.Result{Status: status}
	}
	snapshot := &buildSnapshot{Snapshot: in.Snapshot}
	in.Snapshot = snapshot
	m, err := readMakefile(in, buildMakefile)
	if err != nil {
		return answer(snapshot.failed, err)
	}
	rule, found := m.rule("ci")
	if len(m.files) == 0 || !found {
		return model.Result{Status: model.Measured()}
	}
	missing, err := profileMissing(in, m)
	if err != nil {
		return answer(snapshot.failed, err)
	}
	findings := make([]model.Finding, 0, len(missing))
	for _, target := range missing {
		message := fmt.Sprintf("add %s to the prerequisites of ci, the profile requires it, or run make %s in a scheduled or manually dispatched Actions job", target, target)
		findings = append(findings, finding(rule.file, rule.line, "profile-target-missing", message))
	}
	return model.Result{Status: model.Measured(), Findings: findings}
}

// profileMissing answers the profile targets neither ci nor an on-demand
// lane reaches, consulting the deferred limits only for a target still
// missing, as ciCoverage does for the core table (D2).
func profileMissing(in model.Input, m makefile) ([]string, error) {
	reached, graphLimit := ciReachable(m)
	missing := ciMissing(in.Policy.ProfileData.Targets, reached)
	if len(missing) == 0 {
		return nil, nil
	}
	demand, demandLimit := demandTargets(in)
	missing = ciMissing(missing, demand)
	switch {
	case len(missing) == 0:
		return nil, nil
	case graphLimit != nil:
		return nil, graphLimit
	case demandLimit != nil:
		return nil, demandLimit
	}
	return missing, nil
}
