package hook

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/report"
)

func decisionResult(event model.Event, decision engine.Decision, language string) Result {
	if decision.Verdict == engine.VerdictClean {
		return Result{Outcome: Allow}
	}
	if decision.Reason == "" {
		return rejectionLanguage(event, "cannot judge: engine returned no reason", true, language)
	}
	result := rejectionLanguage(event, decision.Reason, len(decision.Unmeasured) != 0, language)
	result.enforcedFinding = slices.ContainsFunc(decision.Findings, model.Finding.Enforced)
	if event.Shape != model.ShapePost {
		// A denial before execution names the rule through the phrase table
		// of its codes; post feedback keeps the sentence that asks for a fix.
		result.Human = report.Human(language, decision)
	}
	return withSurfaceReason(result, event, decision, language)
}

// surfaceCheck answers whether a check belongs to the families of
// internal/checks/surface, whose reasons M6.6.5 writes as a place and a
// message; the hook core imports no check package, so it reads the prefix.
func surfaceCheck(id string) bool {
	return strings.HasPrefix(id, "surface.") || strings.HasPrefix(id, "docker.") || strings.HasPrefix(id, "cfn.")
}

// surfaceReason names the place first, so an agent reaches the line before
// it reads the fix the message ends with.
func surfaceReason(f model.Finding) string {
	if f.Line == 0 {
		return f.Path + " " + f.Message
	}
	return fmt.Sprintf("%s:%d %s", f.Path, f.Line, f.Message)
}

// withSurfaceReason rewrites the reason and the person's line of a result
// whose first enforced finding, the one the engine's reason names, is of
// the surface families; any other family keeps both as they were.
func withSurfaceReason(result Result, event model.Event, decision engine.Decision, language string) Result {
	i := slices.IndexFunc(decision.Findings, model.Finding.Enforced)
	if i < 0 || !surfaceCheck(decision.Findings[i].Check) {
		return result
	}
	f := decision.Findings[i]
	words := sentencesFor(language)
	sentence := words.blocked
	if event.Shape == model.ShapePost {
		sentence = words.post
	}
	result.Reason = surfaceReason(f)
	result.Human = fmt.Sprintf(sentence, f.Check, f.Path)
	if fix, ok := report.Fix(language, f.Code); ok {
		if f.Code == fileLengthCode {
			fix = withLimit(fix, f.Message)
		}
		result.Human += " " + fix
	}
	return result
}

// fileLengthCode is the code whose fix names the limit, and limitMarker
// precedes the number the file-length messages of the kit name.
const (
	fileLengthCode = "file-length"
	limitMarker    = "the limit is "
)

// withLimit adds the limit a file-length finding names to the shared fix
// of its code, the sentence M6.6.5 writes; a message without the number
// keeps the fix as it is.
func withLimit(fix, message string) string {
	_, after, _ := strings.Cut(message, limitMarker)
	digits := after[:len(after)-len(strings.TrimLeft(after, "0123456789"))]
	if digits == "" {
		return fix
	}
	return strings.TrimSuffix(fix, ".") + "; o limite é " + digits + " linhas."
}

func failure(event model.Event, err error) Result {
	return failureLanguage(event, err, model.DefaultLanguage)
}

func failureLanguage(event model.Event, err error, language string) Result {
	cannot := !errors.Is(err, model.ErrPolicy) && !errors.Is(err, model.ErrEnrollment)
	return rejectionLanguage(event, err.Error(), cannot, language)
}

func rejection(event model.Event, reason string, cannot bool) Result {
	return rejectionLanguage(event, reason, cannot, model.DefaultLanguage)
}

func rejectionLanguage(event model.Event, reason string, cannot bool, language string) Result {
	result := Result{
		Outcome: Deny, Reason: reason, CannotJudge: cannot,
		Human: humanMessage(language, event, reason, cannot),
	}
	if event.Shape == model.ShapePost {
		result.Outcome = Feedback
	}
	return result
}
