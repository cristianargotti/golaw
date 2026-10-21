package cicheck_test

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/assets"
)

func m3CIRule() auditRule {
	return auditRule{
		id: "ci.rendered", rule: "rules/10.5@1",
		match: func(path string) bool { return path == m3CIPath }, inspect: inspectM3CI,
	}
}

func inspectM3CI(path string, data []byte) string {
	if reason := inspectCI(path, data); reason != "" {
		return reason
	}
	steps, err := assets.RunSteps(data)
	if err != nil {
		return "CI must provide literal job steps: " + err.Error()
	}
	root, err := document(data)
	if err != nil {
		return "CI must provide a valid YAML document."
	}
	if reason := inspectM3Jobs(root); reason != "" {
		return reason
	}
	return inspectM3Gate(steps)
}

func inspectM3Jobs(root *yaml.Node) string {
	jobs := children(at(root, "jobs"))
	for i := 0; i+1 < len(jobs); i += 2 {
		id, job := jobs[i].Value, jobs[i+1]
		if value(at(job, "env", "GOFLAGS")) != "-mod=readonly" {
			return id + " must set job GOFLAGS to -mod=readonly."
		}
		if value(at(job, "defaults", "run", "shell")) != "bash" {
			return id + " must set defaults.run.shell to bash."
		}
		if at(job, "continue-on-error") != nil {
			return id + " must remove continue-on-error."
		}
		if reason := inspectM3JobSteps(id, job); reason != "" {
			return reason
		}
	}
	return ""
}

func inspectM3JobSteps(id string, job *yaml.Node) string {
	last := ""
	for index, step := range children(at(job, "steps")) {
		location := fmt.Sprintf("%s step %d", id, index+1)
		if at(step, "if") != nil || at(step, "continue-on-error") != nil {
			return location + " must run without if or continue-on-error."
		}
		if flags := at(step, "env", "GOFLAGS"); flags != nil && value(flags) != "-mod=readonly" {
			return location + " must preserve GOFLAGS -mod=readonly."
		}
		if shell := at(step, "shell"); shell != nil && value(shell) != "bash" {
			return location + " must preserve the bash shell."
		}
		if run := at(step, "run"); run != nil {
			last = strings.TrimSpace(value(run))
		}
	}
	if id != "ci" && last != "git diff --exit-code" {
		return id + " must finish its run steps with git diff --exit-code."
	}
	return ""
}

func inspectM3Gate(steps []assets.RunStep) string {
	count, gate, drift := 0, -1, -1
	for index, step := range steps {
		if step.Job != "ci" {
			continue
		}
		switch strings.TrimSpace(step.Run) {
		case m3GateCommand:
			count++
			gate = index
			if step.Env["GOLAW_ENROLLED"] != m3Enrollment {
				return "ci must run make ci with external enrollment."
			}
		case "git diff --exit-code":
			drift = index
		}
	}
	if count != 1 {
		return "ci must run make ci exactly once with external enrollment."
	}
	if drift <= gate {
		return "ci must run git diff --exit-code after make ci."
	}
	return ""
}
