package cicheck_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestM3RenderedAuditProof(t *testing.T) {
	t.Parallel()
	manifest, err := proof.LoadManifest("proof/m3-ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	registry := registryOf(t, manifest, []auditRule{m3CIRule()})
	for _, failure := range proof.Verify(t.Context(), registry, "proof", manifest) {
		t.Errorf("%+v", failure)
	}
}

func TestM3RenderedCISafety(t *testing.T) {
	t.Parallel()
	for _, capabilities := range m3Capabilities() {
		t.Run(strings.Join(capabilities, ","), func(t *testing.T) {
			t.Parallel()
			data := m3CI(t, capabilities)
			for _, rule := range []auditRule{ciRule(), m3CIRule()} {
				if !rule.match(m3CIPath) {
					t.Fatalf("%s does not select its rendered destination", rule.id)
				}
				if reason := rule.inspect(m3CIPath, data); reason != "" {
					t.Fatalf("%s: %s", rule.id, reason)
				}
			}
			if reason := inspectM3Provisioning(data, capabilities); reason != "" {
				t.Fatal(reason)
			}
		})
	}
}

func TestM3RenderedCIMutations(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join(
		"proof", "testdata", "ci.rendered", "near", ".github", "workflows", "ci.yml",
	))
	if err != nil {
		t.Fatal(err)
	}
	const sha = "08c6903cd8c0fde910a37f88322edcfb5dd907a8"
	for _, test := range []struct {
		name, old, next string
		fail            bool
	}{
		{"near", "make ci", "make ci", false},
		{"tag", sha, "v5", true},
		{"missing version", " # v5.0.0", "", true},
		{"write permission", "contents: read", "contents: write", true},
		{"persisted credential", "persist-credentials: false", "persist-credentials: true", true},
		{"restore omitted", `echo "GOTOOLCHAIN=auto" >> "$GITHUB_ENV"`, "go version", true},
		{"gate removed", "make ci", "go version", true},
		{"readonly removed", "-mod=readonly", "-mod=mod", true},
		{"external value removed", "${{ vars.GOLAW_ENROLLED }}", "", true},
		{"drift removed", "git diff --exit-code", "git status", true},
		{"soft failure", "  ci:\n", "  ci:\n    continue-on-error: true\n", true},
		{"optional gate", "      - run: make ci", "      - if: false\n        run: make ci", true},
		{"harmless metadata", "name: CI", "name: Verification", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			changed := strings.Replace(string(data), test.old, test.next, 1)
			if test.old != test.next && changed == string(data) {
				t.Fatal("mutation did not change the fixture")
			}
			reason := inspectM3CI(m3CIPath, []byte(changed))
			if (reason != "") != test.fail {
				t.Fatalf("reason=%q, want failure=%t", reason, test.fail)
			}
		})
	}
}

func TestM3GateOrdering(t *testing.T) {
	t.Parallel()
	gate := assets.RunStep{Job: "ci", Run: m3GateCommand, Env: map[string]string{"GOLAW_ENROLLED": m3Enrollment}}
	drift := assets.RunStep{Job: "ci", Run: "git diff --exit-code"}
	otherGate, otherDrift := gate, drift
	otherGate.Job, otherDrift.Job = "api", "api"
	for _, test := range []struct {
		name  string
		steps []assets.RunStep
		fail  bool
	}{
		{"near", []assets.RunStep{gate, drift}, false},
		{"empty", nil, true},
		{"gate absent", []assets.RunStep{drift}, true},
		{"gate duplicated", []assets.RunStep{gate, gate, drift}, true},
		{"gate in another job", []assets.RunStep{otherGate, drift}, true},
		{"drift before gate", []assets.RunStep{drift, gate}, true},
		{"drift in another job", []assets.RunStep{gate, otherDrift}, true},
		{"drift both sides", []assets.RunStep{drift, gate, drift}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if reason := inspectM3Gate(test.steps); (reason != "") != test.fail {
				t.Fatalf("reason=%q, want failure=%t", reason, test.fail)
			}
		})
	}
}

func TestM3JobBoundaries(t *testing.T) {
	t.Parallel()
	source := string(m3CI(t, []string{"api", "go"}))
	for _, test := range []struct {
		name, old, next string
		fail            bool
	}{
		{"job eligibility", "if: github.event_name", "if: github.event_name", false},
		{"shell absent", "defaults: {run: {shell: bash} }", "defaults: {}", true},
		{"step shell override", "- run: make ci", "- shell: sh\n        run: make ci", true},
		{"step flags override", "GOLAW_ENROLLED:", "GOFLAGS: -mod=mod\n          GOLAW_ENROLLED:", true},
		{"step soft failure", "- run: make ci", "- continue-on-error: false\n        run: make ci", true},
		{"optional soft failure", "  api:\n", "  api:\n    continue-on-error: true\n", true},
		{"trimmed gate", "- run: make ci", "- run: ' make ci '", false},
		{"fake gate", "- run: make ci", "- run: echo make ci", true},
		{"optional final run", source, source + "      - run: go version\n", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			changed := strings.Replace(source, test.old, test.next, 1)
			if test.old != test.next && changed == source {
				t.Fatal("job mutation did not change its source")
			}
			if reason := inspectM3CI(m3CIPath, []byte(changed)); (reason != "") != test.fail {
				t.Fatalf("reason=%q, want failure=%t", reason, test.fail)
			}
		})
	}
	for _, path := range []string{".github/workflows/release.yml", "ci.yml", m3CIPath + ".tmpl"} {
		if m3CIRule().match(path) {
			t.Errorf("rendered audit selected %s", path)
		}
	}
}
