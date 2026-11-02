package genguard

import (
	"context"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/genheader"
	"github.com/cristianargotti/golaw/internal/model"
)

// The codes and the words of the handwritten escape (M7.9.5, M7.9.6).
const (
	codeHandwrittenReason = "handwritten-reason"
	codeHandwritten       = "handwritten"
	bareMessage           = "adds golaw:handwritten without a reason: write why after the marker, for example // golaw:handwritten the vendor API streams a chunked body."
	markedPrefix          = "is marked golaw:handwritten"
	bareNote              = markedPrefix + " without a reason: write why after the marker"
)

// handwrittenReason denies a bare golaw:handwritten marker the candidate
// introduces: the escape stays reviewable only with its reason, and a bare
// marker the base already carries is left alone so an unrelated edit of an
// old file is never refused.
type handwrittenReason struct{}

// HandwrittenReason is gen.handwritten-reason.
func HandwrittenReason() model.Check { return handwrittenReason{} }

func (handwrittenReason) Spec() model.Spec {
	return spec("gen.handwritten-reason", model.ScopeEvent)
}

// Run judges every Go file a candidate writes, as the snapshot holds it
// after the write.
func (handwrittenReason) Run(_ context.Context, in model.Input) (model.Result, error) {
	if !agentWrite(in) {
		return model.Result{Status: model.NotApplicable(hookOnly)}, nil
	}
	var findings []model.Finding
	var unread []string
	judged := 0
	for _, c := range in.Event.Candidates {
		if c.Op == model.CandidateDelete || !strings.HasSuffix(c.Path, ".go") {
			continue
		}
		judged++
		content, err := in.Snapshot.Read(c.Path)
		if err != nil {
			unread = append(unread, c.Path)
			continue
		}
		line, reason, marked := genheader.HandwrittenAt(content)
		if !marked || reason != "" || baseIsBare(in, c) {
			continue
		}
		findings = append(findings, deny(c.Path, line, codeHandwrittenReason, bareMessage))
	}
	return model.Result{Status: model.Measured(), Scope: strconv.Itoa(judged) + " files", Findings: findings, Unread: unread}, nil
}

// baseIsBare reports whether the file the candidate replaces already
// carries a bare marker. A snapshot that keeps no base, a fixture tree, has
// no file before the candidate, so its marker is the one introduced.
func baseIsBare(in model.Input, c model.Candidate) bool {
	based, ok := in.Snapshot.(model.Based)
	if !ok {
		return false
	}
	from := c.Path
	if c.Op == model.CandidateMove && c.From != "" {
		from = c.From
	}
	content, err := based.Base().Read(from)
	if err != nil {
		return false
	}
	_, reason, marked := genheader.HandwrittenAt(content)
	return marked && reason == ""
}

// handwritten reports every file that took the escape, so the reviewer of
// a scan sees which files golaw never generates.
type handwritten struct{}

// Handwritten is gen.handwritten.
func Handwritten() model.Check { return handwritten{} }

func (handwritten) Spec() model.Spec { return spec("gen.handwritten", model.ScopeFile) }

// Run notes each marked Go file of the targets with its reason. A fixture
// is a proof of the kit, never code of the repository, so it is left out;
// a file it cannot read is skipped, because a note is never enforced and must not
// leave the scan unmeasured.
func (handwritten) Run(_ context.Context, in model.Input) (model.Result, error) {
	if in.Mode != model.ModeScan && in.Mode != model.ModeStaged {
		return model.Result{Status: model.NotApplicable("scan reports the escape")}, nil
	}
	var findings []model.Finding
	judged := 0
	for _, f := range in.Targets {
		if !strings.HasSuffix(f.Path, ".go") || !f.Class.IsText() || f.Class == model.ClassFixture {
			continue
		}
		content, err := in.Snapshot.Read(f.Path)
		if err != nil {
			continue
		}
		judged++
		if line, reason, marked := genheader.HandwrittenAt(content); marked {
			findings = append(findings, note(f.Path, line, reason))
		}
	}
	return model.Result{Status: model.Measured(), Scope: strconv.Itoa(judged) + " files", Findings: findings}, nil
}

// note is the finding of one marked file.
func note(path string, line int, reason string) model.Finding {
	message := bareNote
	if reason != "" {
		message = markedPrefix + ": " + reason
	}
	return model.Finding{Path: path, Line: line, Code: codeHandwritten, Message: message, Severity: model.SeverityNote}
}
