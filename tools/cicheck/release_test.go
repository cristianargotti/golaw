package cicheck_test

import (
	"strings"
	"testing"
)

func TestReleaseWiring(t *testing.T) {
	t.Parallel()
	data := kitBytes(t, ".github/workflows/release.yml")
	if reason := inspectRelease(".github/workflows/release.yml", data); reason != "" {
		t.Fatal(reason)
	}
	for _, mutation := range [][2]string{
		{"needs: gate", "needs: []"},
		{"needs: gate", "needs: gate\n    if: always()"},
		{"bash tools/cicheck/release-gate.sh", "true"},
		{"go run ./tools/acceptance -release", "go run ./tools/acceptance"},
		{"id: acceptance", "id: acceptance\n        continue-on-error: true"},
		{"--certificate-identity", "--certificate-identity-regexp"},
		{"--draft=false", "--draft=true"},
		{"id: attest", "id: omitted"},
		{"tags: ['v*']", "tags: ['*']"},
		{"on:\n  push:", "on:\n  workflow_dispatch:\n  push:"},
		{"id: eligible", "id: eligible\n        continue-on-error: true"},
		{"id: eligible", "id: eligible\n        if: always()"},
		{"id: acceptance", "id: acceptance\n        if: always()"},
		{"ref: ${{ needs.gate.outputs.sha }}", "ref: ${{ github.sha }}"},
		{"id: publish", "id: publish\n        if: always()"},
		{"id: verify", "id: verify\n        continue-on-error: true"},
		{"https://token.actions.githubusercontent.com", "https://issuer.invalid"},
		{"--bundle dist/", "--bundle other/"},
		{"actions/attest-build-provenance@", "actions/upload-artifact@"},
	} {
		changed := strings.Replace(string(data), mutation[0], mutation[1], 1)
		if changed == string(data) {
			t.Fatalf("release mutation did not change its target: %q", mutation[0])
		}
		if inspectRelease(".github/workflows/release.yml", []byte(changed)) == "" {
			t.Fatalf("unguarded release accepted: %q", mutation[1])
		}
	}
}

func TestReleasingProcedure(t *testing.T) {
	t.Parallel()
	requireText(t, "docs/releasing.md",
		"reachable from main",
		"go run ./tools/acceptance -release",
		"docs/live/"+agentID(t)+"-darwin.json",
		"-buildvcs=false",
		"Go "+gateToolchain(t)+", the go-version of release.yml",
		"needs a new live record",
		"immutable candidate",
		"SKIP never counts as PASS",
		"cosign verifies the bundle",
	)
}

// gateToolchain reads the Go version of the gate job, because the gate
// rebuilds the candidate with it and the digest the live record names
// holds only for that toolchain.
func gateToolchain(t *testing.T) string {
	t.Helper()
	root, err := document(kitBytes(t, ".github/workflows/release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range children(at(root, "jobs", "gate", "steps")) {
		if strings.HasPrefix(value(at(step, "uses")), "actions/setup-go@") {
			return value(at(step, "with", "go-version"))
		}
	}
	t.Fatal("the gate job sets up no Go toolchain")
	return ""
}

// A newer merge must neither cancel the push run of an older main commit
// nor replace it while it waits, because the gate refuses a tag whose
// commit has no successful push run; only a pull request yields its runs.
func TestMainPushRunsComplete(t *testing.T) {
	t.Parallel()
	root, err := document(kitBytes(t, ".github/workflows/ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	group := value(at(root, "concurrency", "group"))
	cancel := value(at(root, "concurrency", "cancel-in-progress"))
	if group != "ci-${{ github.event_name == 'pull_request' && github.ref || github.sha }}" ||
		cancel != "${{ github.event_name == 'pull_request' }}" {
		t.Fatalf("concurrency group %q, cancel-in-progress %q", group, cancel)
	}
}
