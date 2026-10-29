package cicheck_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/runner"
)

func TestM3ProvisioningWithoutModule(t *testing.T) {
	t.Parallel()
	const (
		fixed = `with: {go-version: "1.27.0", cache: false}`
		golaw = "go install github.com/cristianargotti/golaw/cmd/golaw@v0.2.0"
	)
	for _, test := range []struct {
		name, old, next string
		caps            []string
		fail            bool
	}{
		{"empty", "make ci", "make ci", nil, false},
		{"empty set without the scanner", golaw, "go version", nil, true},
		{"image only", fixed, fixed, []string{"image"}, false},
		{"module required", fixed, "with: {go-version-file: go.mod}", []string{"image"}, true},
		{"module cache", "cache: false", "cache: true", []string{"image"}, true},
		{"scanner missing", golaw, "go version", []string{"image"}, true},
		{"go module", "go-version-file: go.mod", "go-version-file: go.mod", []string{"go"}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			source := string(m3CI(t, test.caps))
			data := strings.Replace(source, test.old, test.next, 1)
			if test.old != test.next && data == source {
				t.Fatal("provisioning mutation did not change its source")
			}
			reason := inspectM3Provisioning([]byte(data), test.caps)
			if (reason != "") != test.fail {
				t.Fatalf("reason=%q, want failure=%t", reason, test.fail)
			}
		})
	}
}

func inspectM3Provisioning(data []byte, capabilities []string) string {
	runs, err := assets.RunSteps(data)
	if err != nil {
		return "CI jobs must use supported provisioning steps: " + err.Error()
	}
	if reason := inspectCI(m3CIPath, data); reason != "" {
		return reason
	}
	root, err := document(data)
	if err != nil {
		return "CI jobs must provide a valid provisioning document."
	}
	jobs := children(at(root, "jobs"))
	if len(jobs) == 0 {
		return "ci must define a job with pinned golaw provisioning."
	}
	for i := 0; i+1 < len(jobs); i += 2 {
		id, job := jobs[i].Value, jobs[i+1]
		if reason := inspectM3JobSetup(id, job, slices.Contains(capabilities, "go")); reason != "" {
			return reason
		}
		if reason := inspectM3JobInstalls(id, runs, capabilities); reason != "" {
			return reason
		}
	}
	return ""
}

func inspectM3JobSetup(id string, job *yaml.Node, withGo bool) string {
	steps := children(at(job, "steps"))
	setup, count := -1, 0
	for index, step := range steps {
		if strings.HasPrefix(value(at(step, "uses")), "actions/setup-go@") {
			setup, count = index, count+1
		}
	}
	if count != 1 {
		return id + " must contain exactly one setup-go action before installing golaw."
	}
	options := at(steps[setup], "with")
	if withGo {
		if value(at(options, "go-version-file")) != "go.mod" || at(options, "go-version") != nil {
			return id + " must select Go through go-version-file: go.mod."
		}
	} else if reason := inspectM3ModulelessSetup(id, options); reason != "" {
		return reason
	}
	for _, step := range steps[:setup+1] {
		for line := range strings.SplitSeq(value(at(step, "run")), "\n") {
			if m3Installation(strings.TrimSpace(line)) {
				return id + " must run setup-go before installing golaw or check tools."
			}
		}
	}
	return ""
}

func inspectM3ModulelessSetup(id string, options *yaml.Node) string {
	cache := at(options, "cache")
	if at(options, "go-version-file") != nil || value(at(options, "go-version")) != "1.27.0" ||
		cache == nil || cache.Tag != "!!bool" || cache.Value != "false" {
		return id + " without go must set go-version: 1.27.0 and boolean cache: false, without go-version-file."
	}
	return ""
}

func inspectM3JobInstalls(id string, runs []assets.RunStep, capabilities []string) string {
	commands := make(map[string]int)
	for _, step := range runs {
		if step.Job != id {
			continue
		}
		for line := range strings.SplitSeq(step.Run, "\n") {
			commands[strings.TrimSpace(line)]++
		}
	}
	for _, provision := range m3Provisions() {
		want := 0
		if provision.capability == "" || slices.Contains(capabilities, provision.capability) {
			want = 1
		}
		if commands[provision.command] != want {
			return fmt.Sprintf("%s must run %q exactly %d times for its capabilities.", id, provision.command, want)
		}
		delete(commands, provision.command)
	}
	for command := range commands {
		if m3Installation(command) {
			return id + " must remove unrecognized provisioning command: " + command
		}
	}
	return ""
}

type m3Provision struct {
	command    string
	capability string
}

func m3Provisions() []m3Provision {
	provisions := []m3Provision{
		{"go install github.com/cristianargotti/golaw/cmd/golaw@v0.2.0", ""},
		{"npm install --global @redocly/cli@2.5.0", "api"},
		{"npm install --global squawk-cli@2.35.0", "db"},
		{`curl -fsSL https://github.com/hadolint/hadolint/releases/download/v2.15.1/hadolint-linux-x86_64 -o "$RUNNER_TEMP/hadolint"`, "image"},
		{`curl -fsSL https://github.com/aquasecurity/trivy/releases/download/v0.75.0/trivy_0.75.0_Linux-64bit.tar.gz -o "$RUNNER_TEMP/trivy.tar.gz"`, "image"},
		{`install -m 0755 "$RUNNER_TEMP/hadolint" "$RUNNER_TEMP/golaw-bin/hadolint"`, "image"},
		{`tar -xzf "$RUNNER_TEMP/trivy.tar.gz" -C "$RUNNER_TEMP/golaw-bin" trivy`, "image"},
	}
	for _, tool := range runner.Toolchain() {
		if tool.Name != "lefthook" {
			provisions = append(provisions, m3Provision{"go " + strings.Join(tool.InstallArgs(), " "), "go"})
		}
	}
	return provisions
}

func m3Installation(command string) bool {
	for _, prefix := range []string{"go install", "npm install", "curl", "wget", "install", "tar"} {
		if command == prefix || strings.HasPrefix(command, prefix+" ") {
			return true
		}
	}
	return false
}
