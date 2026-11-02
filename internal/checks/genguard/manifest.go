package genguard

import (
	"context"
	"slices"

	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/model"
)

// codeManifestWrite is the finding code of gen.manifest-write.
const codeManifestWrite = "manifest-write"

// manifestWrite denies an agent's write, edit, move or delete of the
// generation manifest (M7.6.1, A42): golaw new and golaw gen write it in one
// deterministic form, so a merge of two branches stays a plain append.
type manifestWrite struct{}

// ManifestWrite is gen.manifest-write.
func ManifestWrite() model.Check { return manifestWrite{} }

func (manifestWrite) Spec() model.Spec { return spec("gen.manifest-write", model.ScopeEvent) }

// Run compares each path the event writes or leaves, the candidates, the
// path a move leaves or the literal targets of a shell command, with the
// manifest path exactly: a check reads no case rule of the filesystem, and
// a case folded match would deny a new file on a case sensitive system.
func (manifestWrite) Run(_ context.Context, in model.Input) (model.Result, error) {
	if !agentEvent(in) {
		return model.Result{Status: model.NotApplicable(hookOnly)}, nil
	}
	var findings []model.Finding
	if slices.Contains(hookPaths(in), manifest.Path) {
		findings = []model.Finding{deny(manifest.Path, 0, codeManifestWrite, "is written only by the golaw CLI: run golaw new <kind> <name> or golaw gen.")}
	}
	return model.Result{Status: model.Measured(), Scope: "1 manifest", Findings: findings}, nil
}
