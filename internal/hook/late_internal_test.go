package hook

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

// lateCase is a result the deadline overtook and whether its reason is an
// enforced finding.
type lateCase struct {
	name     string
	result   Result
	enforced bool
}

// lateCases are the results an event can carry when the deadline overtakes
// it: findings of each severity, with and without unmeasured checks, and
// answers without a finding whose reason may read like one.
func lateCases(event model.Event, language string) []lateCase {
	reason := "text.dashes: em dash (U+2014) in text; use a comma, a colon or a hyphen (notes.md:1)"
	finding := model.Finding{Check: "text.dashes", Path: "notes.md", Line: 1, Code: "em-dash", Severity: model.SeverityDeny}
	unmeasured := []engine.Unmeasured{{Check: "text.trailers", Reason: model.ErrDeadline.Error()}}
	result := func(findings []model.Finding, gaps []engine.Unmeasured, verdict engine.Verdict) Result {
		return decisionResult(event, engine.Decision{Verdict: verdict, Reason: reason, Findings: findings, Unmeasured: gaps}, language)
	}
	denied := []model.Finding{finding}
	finding.Severity = model.SeverityFail
	failed := []model.Finding{finding}
	finding.Severity = model.SeverityNote
	advisory := []model.Finding{finding}
	return []lateCase{
		{"enforced", result(denied, nil, engine.VerdictDeny), true},
		{"enforced with unmeasured", result(denied, unmeasured, engine.VerdictDeny), true},
		{"fail severity", result(failed, nil, engine.VerdictDeny), true},
		{"allow", decisionResult(event, engine.Decision{Verdict: engine.VerdictClean}, language), false},
		{"notice", noticed([]Gap{OverReadBound("large.sql")}, language), false},
		{"unmeasured resembles a finding", result(nil, unmeasured, engine.VerdictDeny), false},
		{"advisory", result(advisory, nil, engine.VerdictClean), false},
		{"advisory with unmeasured", result(advisory, unmeasured, engine.VerdictDeny), false},
		{"snapshot failure resembles a finding", failureLanguage(event, fmt.Errorf("%s: %w", reason, model.ErrUnreadable), language), false},
		{"state failure", failureLanguage(event, fmt.Errorf("read state: %w", model.ErrCannotJudge), language), false},
		{"policy failure", failureLanguage(event, model.ErrPolicy, language), false},
	}
}

// encodedLate answers what lateReply hands the encoder.
func encodedLate(answer completed) (model.Event, Result) {
	var event model.Event
	var result Result
	lateReply(Adapter{Encode: func(gotEvent model.Event, gotResult Result) Reply {
		event, result = gotEvent, gotResult
		return Reply{}
	}}, answer)
	return event, result
}

// assertLatePost holds a late post answer: the finding with the deadline
// notice appended once, or the deadline notice alone.
func assertLatePost(t *testing.T, answer completed, enforced bool, deadline Result) {
	t.Helper()
	gotEvent, got := encodedLate(answer)
	if !reflect.DeepEqual(gotEvent, answer.event) {
		t.Fatalf("event = %+v, want %+v", gotEvent, answer.event)
	}
	if !enforced {
		if got != deadline {
			t.Fatalf("result = %+v, want %+v", got, deadline)
		}
		return
	}
	if got.Outcome != Feedback || got.CannotJudge || got.Notice != "" || got.Reason != answer.result.Reason+" "+deadline.Notice {
		t.Fatalf("finding replaced or incomplete: %+v", got)
	}
	if strings.Count(got.Reason, deadline.Notice) != 1 || got.Human != answer.result.Human {
		t.Fatalf("finding message changed or deadline repeated: %+v", got)
	}
}

// TestLatePostKeepsOnlyEnforcedFindings keeps the feedback of an enforced
// finding at a late post event, in every language, and answers the bare
// notice for any other result.
func TestLatePostKeepsOnlyEnforcedFindings(t *testing.T) {
	t.Parallel()
	event := model.Event{Shape: model.ShapePost, Tool: "Bash", Post: &model.PostResult{}}
	for _, language := range []string{"", model.DefaultLanguage, "en"} {
		selected := language
		if selected == "" {
			selected = model.DefaultLanguage
		}
		deadline := noticed([]Gap{{Cause: causeDeadline}}, selected)
		for _, tc := range lateCases(event, selected) {
			t.Run(language+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				answer := completed{event: event, result: tc.result, language: language}
				assertLatePost(t, answer, tc.enforced, deadline)
			})
		}
	}
}

// TestLatePreAlwaysDenies answers every late pre event with the deadline
// denial, a finding or not.
func TestLatePreAlwaysDenies(t *testing.T) {
	t.Parallel()
	for _, shape := range []model.EventShape{model.ShapeRead, model.ShapeWrite, model.ShapeShell} {
		event := model.Event{Shape: shape}
		for _, tc := range lateCases(event, model.DefaultLanguage) {
			t.Run(shape.String()+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				gotEvent, got := encodedLate(completed{event: event, result: tc.result, language: model.DefaultLanguage})
				if gotEvent.Shape != 0 || got.Outcome != Deny || !got.CannotJudge || got.Notice != "" || !strings.Contains(got.Reason, model.ErrDeadline.Error()) {
					t.Fatalf("late pre result = %+v, event = %+v", got, gotEvent)
				}
			})
		}
	}
}
