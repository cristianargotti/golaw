package hook

import (
	"context"
	"fmt"

	"github.com/cristianargotti/golaw/internal/model"
)

// evaluateOversized answers an event whose input is over the bound from
// the header the adapter reads in its prefix: an unenrolled location is an
// allow, a post event and a write are allowed with a notice naming what
// was not measured, a write of golaw.yml stays a denial, and any other
// event, or one the adapter cannot read or place, is cannot judge.
func evaluateOversized(ctx context.Context, agent string, prefix []byte, adapter Adapter, dependencies Dependencies) completed {
	if dependencies.Begin != nil {
		dependencies.Begin()
	}
	var event model.Event
	err := inputBound()
	if adapter.Oversized != nil {
		event, err = adapter.Oversized(prefix)
	}
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
	return oversizedAnswer(ctx, event, repository, dependencies)
}

// oversizedAnswer is the answer of an enrolled event over the bound once
// its policy is in force: the denial of a write of golaw.yml, the name
// rules over the path a write names, and otherwise the notice of that
// path, or of the changes of a command.
func oversizedAnswer(ctx context.Context, event model.Event, repository Repository, dependencies Dependencies) completed {
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
	gap := Gap{Cause: causeInput}
	if event.Target != "" {
		path, placeErr := Base{Root: repository.Root, Home: dependencies.Home}.Relative(event.Cwd, event.Target)
		if placeErr != nil {
			answer.result = failure(event, placeErr)
			return answer
		}
		gap.Path = path
	}
	if denial := protectPolicy(repository, []model.Candidate{{Path: gap.Path}}); event.Shape == model.ShapeWrite && denial != nil {
		answer.result = failureLanguage(event, denial, answer.language)
		return answer
	}
	if event.Shape != model.ShapeWrite {
		answer.result = noticed([]Gap{gap}, answer.language)
		return answer
	}
	answer.result = judgeOversizedWrite(ctx, event, repository, policy, dependencies, gap)
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
