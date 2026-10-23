package cicheck_test

import (
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// m4LiveFacts are the facts every live record keeps, the required inputs
// of the dispatched job.
func m4LiveFacts() []string {
	return []string{"date", "owner", "environment", "credential_boundary", "spending_limit", "schedule", "retention"}
}

// m4LiveJobFixture is a complete dispatched job of the live lane of agent.
func m4LiveJobFixture(agent string) string {
	variable := strings.ToUpper(agent) + "_API_KEY"
	facts := m4LiveFacts()
	inputs := make([]string, 0, len(facts))
	for _, fact := range facts {
		inputs = append(inputs, "      "+fact+":\n        required: true\n        type: string\n")
	}
	return `
on:
  workflow_dispatch:
    inputs:
` + strings.Join(inputs, "") + `
permissions:
  contents: read

jobs:
  live:
    runs-on: macos-latest
    steps:
      - name: Install the pinned CLI
        run: npm install --global @vendor/` + agent + `@0.163.0
      - name: Record the live session
        env:
          ` + variable + `: ${{ secrets.` + variable + ` }}
        run: go run ./tools/acceptance -live-` + agent + ` "$RUNNER_TEMP/facts.json"
      - name: Keep the record
        run: cat docs/live/` + agent + `-darwin.json
`
}

func TestM4LiveJob(t *testing.T) {
	t.Parallel()
	agent := rulesAgent(t)
	variable := strings.ToUpper(agent) + "_API_KEY"
	fixture := m4LiveJobFixture(agent)
	for _, tt := range []struct{ name, body, want string }{
		{"repository", string(kitBytes(t, ".github/workflows/live-"+agent+".yml")), ""},
		{"near complete", fixture, ""},
		{"corrupted YAML", "on: [", "live job YAML cannot be decoded"},
		{"red a push trigger", strings.Replace(fixture, "  workflow_dispatch:\n", "  push:\n  workflow_dispatch:\n", 1), "the live job runs on manual dispatch alone"},
		{"red a pull request trigger", strings.Replace(fixture, "  workflow_dispatch:\n", "  pull_request:\n  workflow_dispatch:\n", 1), "the live job runs on manual dispatch alone"},
		{"red a fact not required", strings.Replace(fixture, "      retention:\n        required: true\n", "      retention:\n        required: false\n", 1), "the live job must take every fact of the record as a required input"},
		{"red another runner", strings.Replace(fixture, "macos-latest", "ubuntu-latest", 1), "the live job runs on macOS arm64"},
		{"red the secret twice", fixture + "      - run: echo ${{ secrets." + variable + " }}\n", "the secret reaches the lane step alone"},
		{"red a conditional lane", strings.Replace(fixture, "- name: Record the live session", "- if: false\n        name: Record the live session", 1), "missing live lane step"},
		{"red an unpinned CLI", strings.Replace(fixture, "@0.163.0", "@latest", 1), "the live job must install the pinned CLI before the lane"},
		{"red no record kept", strings.Replace(fixture, "        run: cat docs/live/"+agent+"-darwin.json\n", "        run: true\n", 1), "the live job must keep the record in its log"},
	} {
		if got := m4LiveJobReason([]byte(tt.body), agent); got != tt.want {
			t.Errorf("%s: reason=%q, want=%q", tt.name, got, tt.want)
		}
	}
}

// m4LiveJobReason holds the dispatched job of the live lane to section
// 10.4: a person dispatches it, the facts of the record are its inputs, it
// runs on macOS arm64, and the key reaches the lane step alone.
func m4LiveJobReason(data []byte, agent string) string {
	root, err := document(data)
	if err != nil {
		return "live job YAML cannot be decoded"
	}
	triggers := children(at(root, "on"))
	if len(triggers) != 2 || triggers[0].Value != "workflow_dispatch" {
		return "the live job runs on manual dispatch alone"
	}
	for _, fact := range m4LiveFacts() {
		if value(at(root, "on", "workflow_dispatch", "inputs", fact, "required")) != "true" {
			return "the live job must take every fact of the record as a required input"
		}
	}
	jobs := children(at(root, "jobs"))
	if len(jobs) != 2 || value(at(jobs[1], "runs-on")) != "macos-latest" {
		return "the live job runs on macOS arm64"
	}
	if strings.Count(string(data), "secrets.") != 1 {
		return "the secret reaches the lane step alone"
	}
	return m4LiveStepsReason(children(at(jobs[1], "steps")), agent)
}

// m4LiveStepsReason requires the unconditional lane step with the key in
// its own environment, the pinned CLI installed before it, and a later
// step that prints the record into the log of the run.
func m4LiveStepsReason(steps []*yaml.Node, agent string) string {
	variable := strings.ToUpper(agent) + "_API_KEY"
	lane := slices.IndexFunc(steps, func(step *yaml.Node) bool {
		return unconditional(step) && value(at(step, "run")) == `go run ./tools/acceptance -live-`+agent+` "$RUNNER_TEMP/facts.json"` &&
			value(at(step, "env", variable)) == "${{ secrets."+variable+" }}"
	})
	if lane < 0 {
		return "missing live lane step"
	}
	if install := pinnedCLIStep(steps, agent); install < 0 || install > lane {
		return "the live job must install the pinned CLI before the lane"
	}
	kept := slices.IndexFunc(steps, func(step *yaml.Node) bool {
		return strings.Contains(value(at(step, "run")), "docs/live/"+agent+"-darwin.json")
	})
	if kept < lane {
		return "the live job must keep the record in its log"
	}
	return ""
}

func TestM4LiveJobIsInTheProcedure(t *testing.T) {
	t.Parallel()
	agent := rulesAgent(t)
	requireText(t, "docs/live/"+agent+".md",
		".github/workflows/live-"+agent+".yml", strings.ToUpper(agent)+"_API_KEY", "workflow_dispatch", "spending limit")
}
