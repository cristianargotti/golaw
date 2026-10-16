package cicheck_test

import (
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

func ciPath(path string) bool {
	return strings.HasPrefix(path, ".github/workflows/") &&
		(strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".yaml"))
}

func inspectCI(path string, data []byte) string {
	root, err := document(data)
	if err != nil {
		return "CI YAML cannot be decoded."
	}
	permissions := at(root, "permissions")
	if permissions == nil || permissions.Kind != yaml.MappingNode ||
		len(permissions.Content) != 2 || value(at(permissions, "contents")) != "read" {
		return "CI must default to contents read."
	}
	pin := regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_./-]+@[0-9a-f]{40}$`)
	version := regexp.MustCompile(`^# v\d+\.\d+\.\d+$`)
	for _, use := range named(root, "uses") {
		if !pin.MatchString(use.Value) || !version.MatchString(strings.TrimSpace(use.LineComment)) {
			return "Every action needs a SHA and its inline version."
		}
	}
	jobs := children(at(root, "jobs"))
	for i := 0; i+1 < len(jobs); i += 2 {
		if reason := inspectJob(path, jobs[i].Value, jobs[i+1]); reason != "" {
			return reason
		}
	}
	return ""
}

func inspectJob(path, id string, job *yaml.Node) string {
	for i, node := range children(at(job, "permissions")) {
		if i%2 == 0 || node.Value != "write" {
			continue
		}
		if path != ".github/workflows/release.yml" || id != "release" {
			return "Write permission is restricted to publication."
		}
	}
	steps := children(at(job, "steps"))
	for index := range steps {
		if reason := inspectStep(steps, index); reason != "" {
			return reason
		}
	}
	return ""
}

func inspectStep(steps []*yaml.Node, index int) string {
	step := steps[index]
	use := value(at(step, "uses"))
	if strings.HasPrefix(use, "actions/checkout@") {
		persist := at(step, "with", "persist-credentials")
		if persist == nil || persist.Tag != "!!bool" || persist.Value != "false" {
			return "Checkout must disable persisted credentials."
		}
	}
	if strings.HasPrefix(use, "actions/setup-go@") {
		if index+1 >= len(steps) || !restoresAuto(steps[index+1]) {
			return "Every Go setup must immediately restore automatic selection."
		}
	}
	return ""
}

func restoresAuto(step *yaml.Node) bool {
	return at(step, "if") == nil && at(step, "continue-on-error") == nil &&
		strings.TrimSpace(value(at(step, "run"))) ==
			`echo "GOTOOLCHAIN=auto" >> "$GITHUB_ENV"`
}

func ciRule() auditRule {
	return auditRule{
		id: "ci.safety", rule: "rules/10.5@1",
		match: ciPath, inspect: inspectCI,
	}
}
