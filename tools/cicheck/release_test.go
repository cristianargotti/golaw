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
	} {
		changed := strings.Replace(string(data), mutation[0], mutation[1], 1)
		if changed == string(data) {
			t.Fatal("release mutation did not change its target")
		}
		if inspectRelease(".github/workflows/release.yml", []byte(changed)) == "" {
			t.Fatal("unguarded release accepted")
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
		"immutable candidate",
		"SKIP never counts as PASS",
		"cosign verifies the bundle",
	)
}
