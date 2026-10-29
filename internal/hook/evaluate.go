package hook

import (
	"context"
	"errors"
	"fmt"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

// Evaluate establishes authority before reading or reconstructing targets.
func Evaluate(ctx context.Context, agent string, data []byte, adapter Adapter, dependencies Dependencies) (model.Event, Result) {
	answer := evaluateEvent(ctx, agent, data, adapter, dependencies)
	return answer.event, answer.result
}

func evaluateEvent(ctx context.Context, agent string, data []byte, adapter Adapter, dependencies Dependencies) completed {
	return evaluateParsed(ctx, agent, func() (Parsed, error) { return adapter.Parse(data) }, data, dependencies)
}

// evaluateParsed judges an event from its one parse: the agent and the
// enrollment first, then, once the policy in force is loaded, the request
// its decoder answers. data is the input a shell pre event binds its
// baseline to, none for an input over the bound.
func evaluateParsed(ctx context.Context, agent string, parse func() (Parsed, error), data []byte, dependencies Dependencies) completed {
	if dependencies.Begin != nil {
		dependencies.Begin()
	}
	parsed, err := parse()
	if err == nil && parsed.Decode == nil {
		err = fmt.Errorf("%w: the adapter answered no decoder", model.ErrCannotJudge)
	}
	event := parsed.Event
	if err != nil {
		return completed{event: event, result: failure(event, err)}
	}
	if !knownAgent(agent) || event.Agent != agent {
		return completed{event: event, result: failure(event, fmt.Errorf("%w: agent envelope mismatch", model.ErrCannotJudge))}
	}
	repository, err := enroll(ctx, event, parsed.Targets, dependencies)
	if err != nil {
		return completed{event: event, result: failure(event, err)}
	}
	if repository.Enrollment.State == model.EnrollmentNone {
		return completed{event: event, result: Result{Outcome: Allow}}
	}
	run, err := loadExecution(ctx, repository, parsed.Decode, dependencies)
	if err != nil {
		return completed{event: event, result: failureLanguage(event, err, run.language()), language: run.language()}
	}
	return run.answer(ctx, event, data, dependencies)
}

// unmeasured answers a write whose bytes the hook cannot hold, an edit of a
// file over the read bound or a write over the bound of the input:
// golaw.yml stays protected, the name rules judge its files over a view
// that reads them as over the bound, so no rule reads what the base held
// there, and anything else is allowed with the notice of what was not
// measured.
func (run execution) unmeasured(ctx context.Context, dependencies Dependencies) Result {
	event := run.request.Event
	if denial := protectPolicy(run.repository, event.Candidates); denial != nil {
		return failureLanguage(event, denial, run.language())
	}
	view := unheld(run.base, run.request.Gaps)
	in := model.Input{
		Mode: model.ModeHook, Policy: run.policy, Snapshot: view,
		Targets: selectFiles(view, candidatePaths(event.Candidates)), Event: &event,
	}
	result := judgeUnread(ctx, dependencies, in, run.request.Gaps, run.language())
	if failed := run.base.failure(); failed != nil {
		return failureLanguage(event, failed, run.language())
	}
	return result
}

// enroll resolves the repository that governs an event (A3): the first
// enrolled one among the directories enrollmentDirs lists, the one that
// holds the file a write or read leads to first, wherever the session
// runs. A read whose target no enrolled repository holds falls back to
// the session's repository, because the secret boundary guards what enters
// the session's context and the person's credential stores live outside
// every repository; a write there is a positively unenrolled location.
func enroll(ctx context.Context, event model.Event, targets []string, dependencies Dependencies) (Repository, error) {
	var repository Repository
	dirs, err := enrollmentDirs(event, targets, dependencies.Home)
	if err != nil {
		return repository, err
	}
	for _, dir := range dirs {
		repository, err = dependencies.Enroll(ctx, dir, event.Cwd)
		if err != nil || repository.Enrollment.State != model.EnrollmentNone {
			break
		}
	}
	return repository, err
}

// loadExecution loads what an enrolled event is judged with and decodes its
// request with the decoder of its one parse.
func loadExecution(ctx context.Context, repository Repository, decode func(Base, model.Snapshot) (Request, error), dependencies Dependencies) (execution, error) {
	run, snapshot, err := prepare(ctx, repository, dependencies)
	if err != nil {
		return run, err
	}
	request, err := decode(Base{Root: repository.Root, Home: dependencies.Home}, snapshot)
	if failed := snapshot.failure(); failed != nil {
		return run, failed
	}
	if err != nil {
		return run, fmt.Errorf("normalize hook input: %w", err)
	}
	run.base, run.request = snapshot, request
	return run, nil
}

// prepare loads the policy in force of an event's repository, refuses one
// whose policy is not enrolled, sweeps the state and defers the snapshot
// the event's rules read; the event keeps what git answers about the
// commits of its command in a memory of its own.
func prepare(ctx context.Context, repository Repository, dependencies Dependencies) (execution, *deferredSnapshot, error) {
	run := execution{repository: repository, own: &commitMemory{}}
	policy, err := dependencies.Policy(ctx, repository)
	if err != nil {
		return run, nil, fmt.Errorf("load hook policy: %w", err)
	}
	run.policy = policy
	if enrollErr := enrolledPolicy(policy); enrollErr != nil {
		return run, nil, enrollErr
	}
	if sweepErr := sweepState(ctx, dependencies.State); sweepErr != nil && !errors.Is(sweepErr, model.ErrDeadline) {
		run.stateReason = sweepErr.Error()
	}
	return run, deferSnapshot(func() (model.Snapshot, error) { return dependencies.Snapshot(ctx, repository) }), nil
}

func enrolledPolicy(policy *model.EffectivePolicy) error {
	if policy == nil {
		return fmt.Errorf("%w: effective policy is absent", model.ErrPolicy)
	}
	switch policy.Enrollment.State {
	case model.EnrollmentEnrolled:
		return nil
	case model.EnrollmentMissing:
		return fmt.Errorf("%w: enrolled repository is missing golaw.yml", model.ErrPolicy)
	case model.EnrollmentMismatch:
		return fmt.Errorf("%w: golaw.yml enrollment digest mismatch", model.ErrEnrollment)
	case model.EnrollmentNone:
		return unenrolled(policy)
	default:
		return unenrolled(policy)
	}
}

// unenrolled names the state in the denial: discovery did not report this
// location unenrolled, so a policy that carries no positive enrollment is
// a contradiction the hook refuses rather than an allow.
func unenrolled(policy *model.EffectivePolicy) error {
	return fmt.Errorf("%w: effective policy enrollment is %s", model.ErrEnrollment, policy.Enrollment.State)
}

func (run execution) evaluate(ctx context.Context, data []byte, dependencies Dependencies) Result {
	input, gaps, err := run.input(ctx, dependencies)
	if err != nil {
		return failureLanguage(run.request.Event, err, run.language())
	}
	dispatched, err := engine.Dispatch(ctx, dependencies.Registry, input)
	if err == nil {
		err = run.base.failure()
	}
	if err != nil {
		return failureLanguage(run.request.Event, err, run.language())
	}
	decision := engine.Decide(dispatched)
	gaps = withUnread(append(gaps, unapplied(run.request.Event.Candidates)...), decision.Unread)
	fed := run.feedback(ctx, input, decision, dependencies)
	gaps = append(gaps, fed.gaps...)
	result := decisionResult(run.request.Event, decision, run.language())
	if result.Outcome != Allow {
		return fed.apply(withGaps(result, gaps, run.language()), run.language())
	}
	switch run.request.Event.Shape {
	case model.ShapeWrite:
		fresh := run.checkWriteFresh(ctx, dependencies)
		if failed := run.base.failure(); failed != nil {
			fresh = failed
		}
		if fresh != nil {
			return failureLanguage(run.request.Event, fresh, run.language())
		}
	case model.ShapeShell:
		if writes := run.request.Writes; writes == nil || !writes.Intercepted {
			run.saveBefore(ctx, data, dependencies)
		}
	case model.ShapeRead, model.ShapePost:
	}
	return fed.apply(withGaps(result, gaps, run.language()), run.language())
}

func (run execution) language() string {
	if run.policy == nil {
		return model.DefaultLanguage
	}
	return run.policy.Language
}

func knownAgent(agent string) bool {
	switch agent {
	case "claude", "codex", "cursor":
		return true
	default:
		return false
	}
}
