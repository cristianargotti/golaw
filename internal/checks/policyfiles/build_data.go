package policyfiles

// buildStage is an optional git rung and the complete argument terms of the
// binary call that a hook file may carry for it.
type buildStage struct {
	name string
	argv []string
}

// buildFile is a configuration path the build checks read and whether a
// repository must hold it; an optional file is judged only when present.
type buildFile struct {
	path     string
	required bool
}

// buildStages names the evidence a hook file may hold, never a requirement:
// a rung stays armed only by the developer, so no stage is required.
func buildStages() []buildStage {
	const binary, hook = "golaw", "hook"
	return []buildStage{
		{name: "pre-commit", argv: []string{binary, hook, "pre-commit"}},
		{name: "commit-msg", argv: []string{binary, hook, "commit-msg", "{1}"}},
		{name: "pre-push", argv: []string{binary, hook, "pre-push", "{1}", "{2}"}},
	}
}

// buildFiles lists the Makefile, which the go capability requires, and
// lefthook.yml, which is never required because the rungs are optional.
func buildFiles() []buildFile {
	return []buildFile{
		{path: "Makefile", required: true},
		{path: "lefthook.yml"},
	}
}

// demandEvents are the Actions triggers that keep a target moved out of ci
// on demand rather than dropped (section 7).
func demandEvents() []string {
	return []string{"schedule", "workflow_dispatch"}
}
