package hook

import (
	"errors"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/report"
)

// DecisionResult preserves the engine's English reason byte for byte.
func DecisionResult(event model.Event, decision engine.Decision) Result {
	return decisionResult(event, decision, model.DefaultLanguage)
}

func decisionResult(event model.Event, decision engine.Decision, language string) Result {
	if decision.Verdict == engine.VerdictClean {
		return Result{Outcome: Allow}
	}
	if decision.Reason == "" {
		return rejectionLanguage(event, "cannot judge: engine returned no reason", true, language)
	}
	result := rejectionLanguage(event, decision.Reason, len(decision.Unmeasured) != 0, language)
	if event.Shape != model.ShapePost {
		// A denial before execution names the rule through the phrase table
		// of its codes; post feedback keeps the sentence that asks for a fix.
		result.Human = report.Human(language, decision)
	}
	return result
}

func failure(event model.Event, err error) Result {
	return failureLanguage(event, err, model.DefaultLanguage)
}

func failureLanguage(event model.Event, err error, language string) Result {
	cannot := !errors.Is(err, model.ErrPolicy) && !errors.Is(err, model.ErrEnrollment)
	return rejectionLanguage(event, err.Error(), cannot, language)
}

func rejectionLanguage(event model.Event, reason string, cannot bool, language string) Result {
	result := Result{
		Outcome: Deny, Reason: reason, CannotJudge: cannot,
		Human: humanMessage(language, event, reason),
	}
	if event.Shape == model.ShapePost {
		result.Outcome = Feedback
	}
	return result
}
