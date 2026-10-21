package policyfiles

import "github.com/cristianargotti/golaw/internal/model"

const generateCheckTarget = "generate-check"

// GeneratedFresh requires the target that CI can run; executing or grading
// the generator belongs to that target, not to a configuration check.
func GeneratedFresh() model.Check {
	return buildCheck{metadata: spec("generated-fresh", ruleSupply, "go"), judge: judgeGeneratedFresh}
}

// A present definition is sufficient, while a missing root belongs to
// policy.makefile and uncertain definitions cannot justify another finding.
func judgeGeneratedFresh(in model.Input) model.Result {
	snapshot := &buildSnapshot{Snapshot: in.Snapshot}
	in.Snapshot = snapshot
	m, err := readMakefile(in, buildMakefile)
	if err != nil {
		return answer(snapshot.failed, err)
	}
	if len(m.files) == 0 {
		return model.Result{Status: model.Measured()}
	}
	if _, found := m.rule(generateCheckTarget); found {
		return model.Result{Status: model.Measured()}
	}
	if err := ciIncludes(m); err != nil {
		return answer(buildMakefile, err)
	}
	if err := makeTargetLimit(m, generateCheckTarget); err != nil {
		return answer(buildMakefile, err)
	}
	return model.Result{Status: model.Measured(), Findings: []model.Finding{
		finding(buildMakefile, 0, "generate-check-missing",
			"add the generate-check target to Makefile or an included file"),
	}}
}
