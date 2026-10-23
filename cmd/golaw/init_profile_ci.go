package main

import (
	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// ciWorkflow is the managed CI lane of a repository, whose make ci runs
// golaw scan and so needs the bundle a pin names.
const ciWorkflow = ".github/workflows/ci.yml"

// profileCILine names what the remote CI needs once the policy pins a
// profile and init keeps the managed CI lane: its make ci runs golaw scan,
// which refuses without the bundle, so the lane installs the pinned
// version first, with a credential that reads the bundle (A19, Decision
// 4). A profile that leaves .github/ to the platform keeps no lane and
// prints nothing, and so does core only.
func profileCILine(plan initPlan) string {
	f, err := policy.Decode(plan.data)
	if err != nil || f.Profile == nil || !keepsCI(plan.templates) {
		return ""
	}
	command := "golaw profile install " + f.Profile.Source + "@" + f.Profile.Version
	if plan.policy != nil && plan.policy.Language != model.DefaultLanguage {
		return "The CI of " + ciWorkflow + " runs make ci, whose golaw scan needs profile " + f.Profile.Name +
			": run " + command + " in the CI, with a credential that reads the bundle, before make ci."
	}
	return "O CI de " + ciWorkflow + " roda make ci, cujo golaw scan precisa do perfil " + f.Profile.Name +
		": execute " + command + " no CI, com uma credencial que leia o bundle, antes de make ci."
}

// keepsCI reports whether the template plan writes the CI lane or keeps
// managing it.
func keepsCI(plan bootstrap.TemplatePlan) bool {
	for _, change := range plan.Changes {
		if change.Path == ciWorkflow {
			return true
		}
	}
	for _, managed := range plan.Managed {
		if managed.Path == ciWorkflow {
			return true
		}
	}
	return false
}
