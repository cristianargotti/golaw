package cicheck_test

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
)

func inspectPlugin(agent string, data []byte) string {
	root, err := document(data)
	if err != nil {
		return "Plugin CI cannot be decoded."
	}
	job := at(root, "jobs", agent+"-plugin")
	if job == nil || at(job, "if") != nil || at(job, "continue-on-error") != nil {
		return "The plugin job must be required."
	}
	next := 0
	for _, step := range children(at(job, "steps")) {
		run := strings.TrimSpace(value(at(step, "run")))
		if !pluginStep(agent, next, run) {
			continue
		}
		if at(step, "if") != nil || at(step, "continue-on-error") != nil {
			return "Plugin validation cannot be optional."
		}
		next++
	}
	if next != 2 {
		return "Pinned installation and strict validation are required."
	}
	return ""
}

func pluginStep(agent string, index int, command string) bool {
	switch index {
	case 0:
		return pinnedPluginHost(agent, command)
	case 1:
		return command == agent+" plugin validate --strict agents/"+agent
	default:
		return false
	}
}

func pinnedPluginHost(agent, command string) bool {
	pattern := regexp.MustCompile(
		`^npm install --global @[a-z0-9-]+/` + regexp.QuoteMeta(agent) +
			`-code@([0-9]+)\.([0-9]+)\.([0-9]+)$`,
	)
	match := pattern.FindStringSubmatch(command)
	if match == nil {
		return false
	}
	var version [3]int
	for index := range version {
		number, err := strconv.Atoi(match[index+1])
		if err != nil {
			return false
		}
		version[index] = number
	}
	return slices.Compare(version[:], []int{2, 1, 295}) >= 0
}

func pluginRule(agent string) auditRule {
	return auditRule{
		id: "ci.plugin", rule: "rules/10.5@1",
		match: func(path string) bool { return path == ".github/workflows/ci.yml" },
		inspect: func(_ string, data []byte) string {
			return inspectPlugin(agent, data)
		},
	}
}
