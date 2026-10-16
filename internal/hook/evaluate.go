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
	event, err := adapter.Header(data)
	if err != nil {
		return event, failure(event, err)
	}
	if !knownAgent(agent) || event.Agent != agent {
		return event, failure(event, fmt.Errorf("%w: agent envelope mismatch", model.ErrCannotJudge))
	}
	repository, err := enroll(ctx, event, dependencies)
	if err != nil {
		return event, failure(event, err)
	}
	if repository.Enrollment.State == model.EnrollmentNone {
		return event, Result{Outcome: Allow}
	}
	run, err := loadExecution(ctx, repository, data, adapter, dependencies)
	if err != nil {
		return event, failureLanguage(event, err, run.language())
	}
	if run.request.Denial != "" {
		return event, rejectionLanguage(event, run.request.Denial, false, run.language())
	}
	return run.request.Event, run.evaluate(ctx, data, dependencies)
}

// enroll resolves the repository that governs an event (A3): the first
// enrolled one among the directories enrollmentDirs lists, the one that
// holds the file a write or read leads to first, wherever the session
// runs. A read whose target no enrolled repository holds falls back to
// the session's repository, because the secret boundary guards what enters
// the session's context and the person's credential stores live outside
// every repository; a write there is a positively unenrolled location.
func enroll(ctx context.Context, event model.Event, dependencies Dependencies) (Repository, error) {
	var repository Repository
	dirs, err := enrollmentDirs(event, dependencies.Home)
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

func loadExecution(ctx context.Context, repository Repository, data []byte, adapter Adapter, dependencies Dependencies) (execution, error) {
	run := execution{repository: repository}
	policy, err := dependencies.Policy(ctx, repository)
	if err != nil {
		return run, fmt.Errorf("load hook policy: %w", err)
	}
	run.policy = policy
	if enrollErr := enrolledPolicy(policy); enrollErr != nil {
		return run, enrollErr
	}
	if sweepErr := sweepState(ctx, dependencies.State); sweepErr != nil && !errors.Is(sweepErr, model.ErrDeadline) {
		run.stateReason = sweepErr.Error()
	}
	snapshot, err := dependencies.Snapshot(ctx, repository)
	if err != nil {
		return run, fmt.Errorf("%w: snapshot: %w", model.ErrCannotJudge, err)
	}
	request, err := adapter.Decode(data, Base{Root: repository.Root, Home: dependencies.Home}, snapshot)
	if err != nil {
		return run, fmt.Errorf("normalize hook input: %w", err)
	}
	run.base, run.request = snapshot, request
	return run, nil
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
	input, unmeasured, err := run.input(ctx, dependencies)
	if err != nil {
		return failureLanguage(run.request.Event, err, run.language())
	}
	dispatched, err := engine.Dispatch(ctx, dependencies.Registry, input)
	if err != nil {
		return failureLanguage(run.request.Event, err, run.language())
	}
	result := decisionResult(run.request.Event, engine.Decide(dispatched), run.language())
	if result.Outcome != Allow {
		return result
	}
	if unmeasured != "" {
		return rejectionLanguage(run.request.Event,
			"detection not measured: "+unmeasured, false, run.language())
	}
	switch run.request.Event.Shape {
	case model.ShapeWrite:
		if err := run.checkWriteFresh(ctx, dependencies); err != nil {
			return failureLanguage(run.request.Event, err, run.language())
		}
	case model.ShapeShell:
		run.saveBefore(ctx, data, dependencies)
	case model.ShapeRead, model.ShapePost:
	}
	return result
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
