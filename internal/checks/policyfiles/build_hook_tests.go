package policyfiles

import (
	"path"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// HookTests keeps the adjacent-test convention scoped to enrolled profiles.
func HookTests() model.Check {
	return buildCheck{metadata: spec("hook-tests", ruleTests), judge: judgeHookTests}
}

// judgeHookTests answers before touching the snapshot when no profile is
// enrolled, because the convention is profile data and a core repository
// ships no hook script the rule could ask a test of.
func judgeHookTests(in model.Input) model.Result {
	if status, ok := profiled(in); !ok {
		return model.Result{Status: status}
	}
	result := model.Result{Status: model.Measured()}
	for _, rule := range hookTestRules() {
		for _, script := range model.Select(in.Snapshot, rule.matches) {
			pair := judgeHookTestPair(in, rule, script.Path)
			if pair.Status.Kind != model.StatusMeasured || len(pair.Unread) != 0 {
				return pair
			}
			result.Findings = append(result.Findings, pair.Findings...)
		}
	}
	return result
}

// judgeHookTestPair requires only that the paired test exists: running or
// grading it belongs to the hooks-test target of the profile, never to a
// snapshot judgment.
func judgeHookTestPair(in model.Input, rule hookTestRule, script string) model.Result {
	_, found, err := read(in, script)
	if err != nil {
		return answer(script, err)
	}
	result := model.Result{Status: model.Measured()}
	if !found {
		return result
	}
	testPath := strings.TrimSuffix(script, rule.scriptSuffix) + rule.testSuffix
	_, found, err = read(in, testPath)
	if err != nil {
		return answer(testPath, err)
	}
	if !found {
		result.Findings = []model.Finding{finding(script, 0, "hook-test-missing",
			"add "+testPath+" beside "+script)}
	}
	return result
}

// matches selects scripts by their exact parent directory, so a nested
// script, a test file and every other extension stay outside the convention
// in silence instead of becoming a guessed pairing.
func (r hookTestRule) matches(file string) bool {
	return path.Dir(file) == r.directory && strings.HasSuffix(file, r.scriptSuffix) &&
		!strings.HasSuffix(file, r.testSuffix)
}
