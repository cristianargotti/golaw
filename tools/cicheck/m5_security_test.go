package cicheck_test

import (
	"strings"
	"testing"
)

const m5SecurityPath = "docs/security.md"

// m5Residuals are the distribution limits docs/security.md states.
func m5Residuals() []string {
	return []string{
		"## Distribution and profile bundles",
		"golaw doctor --release <dir> verifies a downloaded binary inside the kit, with no network",
		"A red result exits 1.",
		"The trust root is embedded.",
		"a rotation of either reaches a person only through a kit release",
		"--trust-root <dir> replaces them with roots.crt and rekor.pub of a directory",
		"The inclusion proof is not verified.",
		"the body of the log entry is compared with the bundle",
		"Only the v0.3 bundle form is read",
		"The verifier reads no clock.",
		"A go install or source build is never verified.",
		"The attestation verifies only while the repository is public at the tag",
		"A profile bundle on disk is trusted by its digest alone.",
		"The bundle carries no signature",
		"Hooks never fetch a profile, so no hook reaches the network.",
		"a go build in a clean checkout of a tag records no module sum",
		"The agent guards do not stop an agent from running golaw init --profile <name> --apply",
	}
}

func TestM5DistributionResiduals(t *testing.T) {
	t.Parallel()
	text := string(kitBytes(t, m5SecurityPath))
	required := m5Residuals()
	forbidden := []string{
		"The inclusion proof is verified.",
		"doctor verifies a go install build",
		"A profile bundle is signed",
		"the body of the log entry is not compared",
	}
	rows := []struct {
		name, text string
		want       bool
	}{
		{"repository", text, true},
		{"red-missing-trust-root", strings.Replace(text, required[3], "", 1), false},
		{"red-missing-source-build", strings.Replace(text, required[10], "", 1), false},
		{"red-inclusion-overclaim", text + "\n" + forbidden[0] + "\n", false},
		{"red-signed-bundle-overclaim", text + "\n" + forbidden[2] + ".\n", false},
		{"red-body-not-compared", text + "\n" + forbidden[3] + " with the bundle.\n", false},
		{"red-missing-agent-limit", strings.Replace(text, required[16], "", 1), false},
		{"near-pointer", text + "\nSee releasing.md for the dry run of the release gate.\n", true},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if got := m4TextOK(row.text, required, forbidden); got != row.want {
				t.Fatalf("distribution residuals = %v, want %v", got, row.want)
			}
		})
	}
}
