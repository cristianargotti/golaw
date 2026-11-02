package genguard

import (
	"context"
	"errors"
	"strconv"

	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/model"
)

// generatedWrite denies an agent's write of a generated file (M7.9.2): the
// file as the base holds it carries the Go convention header, or the
// manifest lists it as regenerable, so a write that strips the header first
// is denied all the same. A shell command is judged by its literal targets;
// one that names no generated file, golaw gen itself, yields nothing.
type generatedWrite struct{}

// GeneratedWrite is gen.generated-write, the read only denial of A46 and A47.
func GeneratedWrite() model.Check { return generatedWrite{} }

func (generatedWrite) Spec() model.Spec { return spec("gen.generated-write", model.ScopeEvent) }

// Run judges every path the candidates, or the shell command, write or
// leave.
func (generatedWrite) Run(_ context.Context, in model.Input) (model.Result, error) {
	if !agentEvent(in) {
		return model.Result{Status: model.NotApplicable(hookOnly)}, nil
	}
	paths := hookPaths(in)
	if len(paths) == 0 {
		// A shell command that names no file reads nothing, so the
		// commands of M1 keep their cost.
		return model.Result{Status: model.Measured(), Scope: "0 files"}, nil
	}
	base := baseSnapshot(in)
	known := manifestOf(base)
	var findings []model.Finding
	var unread []string
	// A shell target the hook cannot read is left to the post feedback,
	// which names it already, so a command never gains a notice it had
	// none of before the gen family judged shell commands.
	notice := in.Event.Shape == model.ShapeWrite
	for _, p := range paths {
		f, over := judgePath(in, base, known, p)
		if over && notice {
			unread = append(unread, p)
		}
		if f != nil {
			findings = append(findings, *f)
		}
	}
	return model.Result{Status: model.Measured(), Scope: strconv.Itoa(len(paths)) + " files", Findings: findings, Unread: unread}, nil
}

// judgedPaths are the paths of the candidates and, for a move, the path it
// leaves, because moving a generated file away edits it as surely as a
// delete does.
func judgedPaths(candidates []model.Candidate) []string {
	var paths []string
	for _, c := range candidates {
		paths = append(paths, c.Path)
		if c.Op == model.CandidateMove && c.From != "" {
			paths = append(paths, c.From)
		}
	}
	return paths
}

// judgePath answers the finding of one path, or true when its base could
// not be read, over the read bound or unreadable, and so is left unread: a
// guard that cannot read the header must not deny, and the hook names the
// file in a notice instead. A regenerable path the manifest lists is denied
// by its name alone, whatever its base.
func judgePath(in model.Input, base model.Snapshot, known *manifest.Manifest, path string) (*model.Finding, bool) {
	content, _, err := baseContent(in, path)
	unread := err != nil
	exists := func(p string) bool {
		_, err := base.Read(p)
		return !errors.Is(err, model.ErrNotInSnapshot)
	}
	reason, ok := generatedReason(path, content, known, in.Policy.Generate, exists)
	if !ok {
		return nil, unread
	}
	f := deny(path, 0, codeGeneratedWrite, reason)
	return &f, false
}
