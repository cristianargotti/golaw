package hook

import (
	"context"
	"fmt"
	"io"

	"github.com/cristianargotti/golaw/internal/model"
)

// evaluateOversized answers an event whose input is over the bound from
// the header the adapter reads in the whole input: an unenrolled location
// is an allow, a post event is judged from its header as any post event,
// a write is judged by the name rules over its path and otherwise allowed
// with a notice naming it, a write of golaw.yml stays a denial, and any
// other event, or one the adapter cannot read or place, is cannot judge.
func evaluateOversized(ctx context.Context, agent string, input io.Reader, adapter Adapter, dependencies Dependencies) completed {
	if dependencies.Begin != nil {
		dependencies.Begin()
	}
	event, err := oversizedHeader(adapter, input)
	if err != nil || !knownAgent(agent) || event.Agent != agent ||
		event.Shape != model.ShapePost && (event.Shape != model.ShapeWrite || event.Target == "") {
		return completed{event: event, result: failure(event, inputBound())}
	}
	repository, err := enroll(ctx, event, dependencies)
	if err != nil {
		return completed{event: event, result: failure(event, err)}
	}
	if repository.Enrollment.State == model.EnrollmentNone {
		return completed{event: event, result: Result{Outcome: Allow}}
	}
	if event.Shape == model.ShapePost {
		return oversizedPost(ctx, event, repository, dependencies)
	}
	return oversizedWrite(ctx, event, repository, dependencies)
}

// oversizedHeader reads the header of an input over the bound with the
// adapter, then reads the rest and drops it, so the host that writes it
// never meets a closed pipe; without a reader the input is the bound.
func oversizedHeader(adapter Adapter, input io.Reader) (model.Event, error) {
	event, err := model.Event{}, inputBound()
	if adapter.Oversized != nil {
		event, err = adapter.Oversized(input)
	}
	if _, drained := io.Copy(io.Discard, input); drained != nil && err == nil {
		err = fmt.Errorf("%w: read input: %w", model.ErrCannotJudge, drained)
	}
	return event, err
}

// oversizedPost judges a post event over the bound from its header: the
// detection of a command reads git and the baseline of its pre event,
// never the event, so it runs as for any post event, and the path a write
// names is its target.
func oversizedPost(ctx context.Context, event model.Event, repository Repository, dependencies Dependencies) completed {
	run, snapshot, err := prepare(ctx, repository, dependencies)
	answer := completed{event: event, language: run.language()}
	if err != nil {
		answer.result = failureLanguage(event, err, answer.language)
		return answer
	}
	request := Request{Event: event}
	if event.Target != "" {
		path, placeErr := Base{Root: repository.Root, Home: dependencies.Home}.Relative(event.Cwd, event.Target)
		if placeErr != nil {
			answer.result = failure(event, placeErr)
			return answer
		}
		request.Hints = []string{path}
	}
	run.base, run.request = snapshot, request
	answer.result = run.evaluate(ctx, nil, dependencies)
	return answer
}

// oversizedWrite is the answer of an enrolled write over the bound once
// its policy is in force: the denial of a write of golaw.yml, and the name
// rules over the path it names.
func oversizedWrite(ctx context.Context, event model.Event, repository Repository, dependencies Dependencies) completed {
	answer := completed{event: event, language: model.DefaultLanguage}
	policy, err := dependencies.Policy(ctx, repository)
	if err == nil {
		answer.language = policy.Language
		err = enrolledPolicy(policy)
	}
	if err != nil {
		answer.result = failureLanguage(event, fmt.Errorf("load hook policy: %w", err), answer.language)
		return answer
	}
	path, err := Base{Root: repository.Root, Home: dependencies.Home}.Relative(event.Cwd, event.Target)
	if err != nil {
		answer.result = failure(event, err)
		return answer
	}
	if denial := protectPolicy(repository, []model.Candidate{{Path: path}}); denial != nil {
		answer.result = failureLanguage(event, denial, answer.language)
		return answer
	}
	answer.result = judgeOversizedWrite(ctx, event, repository, policy, dependencies, Gap{Path: path, Cause: causeInput})
	return answer
}

// judgeOversizedWrite judges a write over the input bound by the path it
// names: the snapshot reads that path as over the bound, so the name rules
// judge it and no rule reads what the file held before the write.
func judgeOversizedWrite(ctx context.Context, event model.Event, repository Repository, policy *model.EffectivePolicy, dependencies Dependencies, gap Gap) Result {
	base := deferSnapshot(func() (model.Snapshot, error) { return dependencies.Snapshot(ctx, repository) })
	view := unreadView{base: base, files: map[string]model.File{gap.Path: overBound(base, gap.Path)}}
	op := model.CandidateCreate
	if base.Class(gap.Path) != model.ClassUnknown {
		op = model.CandidateReplace
	}
	event.Candidates = []model.Candidate{{Path: gap.Path, Op: op}}
	in := model.Input{Mode: model.ModeHook, Policy: policy, Snapshot: view, Targets: []model.File{view.files[gap.Path]}, Event: &event}
	result := judgeUnread(ctx, dependencies, in, []Gap{gap}, policy.Language)
	if failed := base.failure(); failed != nil {
		return failureLanguage(event, failed, policy.Language)
	}
	return result
}
