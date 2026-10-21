package proof

import (
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// Condition is where a check runs: the scope the engine dispatches it in
// and the capabilities it needs, as section 6 gives its family the
// Condition column and the contract of M3 gives each policy check.
type Condition struct {
	Scope model.Scope
	Needs []string
}

// Each family of section 6 and the one capability its checks need.
func familyConditions() map[string]Condition {
	return map[string]Condition{
		"text":   {Scope: model.ScopeFile},
		"paths":  {Scope: model.ScopeFile},
		"goast":  {Scope: model.ScopeFile, Needs: []string{"go"}},
		"shell":  {Scope: model.ScopeEvent},
		"commit": {Scope: model.ScopeCommit},
	}
}

// checkConditions are the checks whose condition is their own: the two that
// differ from their family, and every policy check, tree scoped (D1) and
// conditioned per check, because the family has no single capability.
func checkConditions() map[string]Condition {
	tree := func(needs ...string) Condition { return Condition{Scope: model.ScopeTree, Needs: needs} }
	return map[string]Condition{
		"paths.secret-material":      {Scope: model.ScopeEvent},
		"goast.raw-error-response":   {Scope: model.ScopeFile, Needs: []string{"go", "api"}},
		"policy.golangci":            tree("go"),
		"policy.ruleguard":           tree("go"),
		"policy.gitleaks-generic":    tree(),
		"policy.gotoolchain-local":   tree("go"),
		"policy.makefile":            tree("go"),
		"policy.ci-target":           tree("go"),
		"policy.generated-fresh":     tree("go"),
		"policy.hook-tests":          tree(),
		"policy.dependency-records":  tree("go"),
		"policy.dependabot-cooldown": tree(),
		"policy.deps-age-config":     tree("go"),
		"policy.image-digest":        tree("image"),
		"policy.codeowners-foreign":  tree(),
		"policy.openapi-assertions":  tree("api"),
		"policy.squawk-config":       tree("db"),
	}
}

// ConditionOf answers the condition of a check id: its own row, else the
// row of its family; false for an id neither holds, which a new check meets
// until its condition is written here.
func ConditionOf(id string) (Condition, bool) {
	if c, ok := checkConditions()[id]; ok {
		return c, true
	}
	family, _, _ := strings.Cut(id, ".")
	c, ok := familyConditions()[family]
	return c, ok
}

// runsIn are the dispatches of section 1.2 and D1 each scope runs in: file
// checks over a scan, the staged scan and a hook write or post, tree checks
// over the two scans only, event checks in every hook shape and commit
// checks in the commit rung alone.
func runsIn(scope model.Scope) []string {
	switch scope {
	case model.ScopeFile:
		return []string{inScan, inStaged, inWrite, inPost}
	case model.ScopeTree:
		return []string{inScan, inStaged}
	case model.ScopeEvent:
		return []string{inWrite, inRead, inShell, inPost}
	case model.ScopeCommit:
		return []string{inCommit}
	default:
		return nil
	}
}
