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
	repository, err := dependencies.Enroll(ctx, event.Cwd)
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
	request, err := adapter.Decode(data, repository.Root, snapshot)
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

// unenrolled is the denial of a policy whose repository is not enrolled.
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
