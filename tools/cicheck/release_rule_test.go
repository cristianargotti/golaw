package cicheck_test

import (
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

func stepByID(job *yaml.Node, id string) *yaml.Node {
	for _, step := range children(at(job, "steps")) {
		if value(at(step, "id")) == id {
			return step
		}
	}
	return nil
}

func inspectRelease(_ string, data []byte) string {
	root, err := document(data)
	if err != nil {
		return "Release YAML cannot be decoded."
	}
	on := at(root, "on")
	if on == nil || len(on.Content) != 2 ||
		!slices.Equal(scalarList(at(on, "push", "tags")), []string{"v*"}) {
		return "Release must be triggered only by version tags."
	}
	gate := at(root, "jobs", "gate")
	release := at(root, "jobs", "release")
	if value(at(release, "needs")) != "gate" || at(release, "if") != nil {
		return "Publication must require a successful eligibility job."
	}
	eligible := stepByID(gate, "eligible")
	if value(at(eligible, "run")) != "bash tools/cicheck/release-gate.sh" ||
		at(eligible, "if") != nil || at(eligible, "continue-on-error") != nil {
		return "Release eligibility must run without a bypass."
	}
	acceptance := stepByID(gate, "acceptance")
	if value(at(acceptance, "run")) != "go run ./tools/acceptance -release" ||
		at(acceptance, "if") != nil || at(acceptance, "continue-on-error") != nil {
		return "The release rows and the dated live record must gate the release."
	}
	checkout := stepByID(release, "checkout")
	if value(at(checkout, "with", "ref")) != "${{ needs.gate.outputs.sha }}" {
		return "Publication must check out the eligible commit."
	}
	return inspectPublication(release)
}

func inspectPublication(job *yaml.Node) string {
	var order []string
	for _, step := range children(at(job, "steps")) {
		id := value(at(step, "id"))
		if slices.Contains([]string{"draft", "verify", "attest", "publish"}, id) {
			if at(step, "if") != nil || at(step, "continue-on-error") != nil {
				return "Publication verification cannot be optional."
			}
			order = append(order, id)
		}
	}
	if !slices.Equal(order, []string{"draft", "verify", "attest", "publish"}) {
		return "Draft signing and attestation must precede publication."
	}
	verify := value(at(stepByID(job, "verify"), "run"))
	identity := `"https://github.com/cristianargotti/golaw/.github/workflows/release.yml@${GITHUB_REF}"`
	if !strings.Contains(verify, "--certificate-identity \\\n  "+identity) ||
		!strings.Contains(verify, "https://token.actions.githubusercontent.com") ||
		!strings.Contains(verify, "--bundle dist/checksums.txt.sigstore.json") {
		return "The release certificate identity must be exact."
	}
	attest := stepByID(job, "attest")
	if !strings.HasPrefix(value(at(attest, "uses")), "actions/attest-build-provenance@") {
		return "Binary provenance attestation is required."
	}
	publish := value(at(stepByID(job, "publish"), "run"))
	if publish != `gh release edit "$GITHUB_REF_NAME" --repo "$GITHUB_REPOSITORY" --draft=false` {
		return "Only the verified draft may be published."
	}
	return ""
}

func releaseRule() auditRule {
	return auditRule{
		id: "release.pipeline", rule: "rules/10.5@1",
		match:   func(path string) bool { return path == ".github/workflows/release.yml" },
		inspect: inspectRelease,
	}
}
