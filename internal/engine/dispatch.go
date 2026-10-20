package engine

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/cristianargotti/golaw/internal/model"
)

// Outcome is the result of one registered check, run or not.
type Outcome struct {
	Spec   model.Spec
	Result model.Result
}

// Run is the outcome of every registered check, in registry order.
type Run struct {
	Mode     model.Mode
	Outcomes []Outcome
}

// Findings returns every finding of the run in deterministic order.
func (r *Run) Findings() []model.Finding {
	var out []model.Finding
	for i := range r.Outcomes {
		out = append(out, r.Outcomes[i].Result.Findings...)
	}
	model.SortFindings(out)
	return out
}

// Unmeasured returns the outcomes of the checks that applied but could not
// run; each one fails the run.
func (r *Run) Unmeasured() []Outcome {
	var out []Outcome
	for i := range r.Outcomes {
		if r.Outcomes[i].Result.Status.Kind == model.StatusUnmeasured {
			out = append(out, r.Outcomes[i])
		}
	}
	return out
}

// Unread returns the files over the read bound the measured checks of the
// run left unread, sorted and each once.
func (r *Run) Unread() []string {
	var out []string
	for i := range r.Outcomes {
		if r.Outcomes[i].Result.Status.Kind == model.StatusMeasured {
			out = append(out, r.Outcomes[i].Result.Unread...)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// Dispatch runs every applicable check of the registry over the input and
// records one outcome per registered check. A check that panics, errs or
// forgets its status is Unmeasured, never silent; once the context is done
// the remaining checks are Unmeasured with the deadline as the reason.
func Dispatch(ctx context.Context, reg *Registry, in model.Input) (*Run, error) {
	if err := validate(reg, in); err != nil {
		return nil, err
	}
	run := &Run{Mode: in.Mode, Outcomes: make([]Outcome, 0, reg.Len())}
	for _, c := range reg.checks {
		spec := c.Spec()
		run.Outcomes = append(run.Outcomes, Outcome{Spec: spec, Result: dispatchOne(ctx, c, spec, in)})
	}
	return run, nil
}

func dispatchOne(ctx context.Context, c model.Check, spec model.Spec, in model.Input) model.Result {
	if err := ctx.Err(); err != nil {
		return model.Result{Status: model.Unmeasured("deadline: " + err.Error())}
	}
	if status, ok := Applicability(spec, in); !ok {
		return model.Result{Status: status}
	}
	in.Options = resolveOptions(spec, in.Policy)
	return unread(in.Mode, normalize(spec, in.Policy, runOne(ctx, c, in)))
}

// unread holds a check to the read bound by mode: a hook keeps the files
// the check left unread beside its measured result, to name them in a
// notice, and every other mode counts the check unmeasured, never clean.
func unread(mode model.Mode, res model.Result) model.Result {
	if len(res.Unread) == 0 || mode == model.ModeHook || res.Status.Kind != model.StatusMeasured {
		return res
	}
	res.Status = model.Unmeasured(fmt.Sprintf("read %s: %v", res.Unread[0], model.ErrTooLarge))
	return res
}

// validate refuses an input Dispatch cannot judge, as a usage error.
func validate(reg *Registry, in model.Input) error {
	switch {
	case reg == nil:
		return fmt.Errorf("%w: nil registry", model.ErrUsage)
	case !in.Mode.Valid():
		return fmt.Errorf("%w: mode %d is not scan, staged, hook or commit", model.ErrUsage, int(in.Mode))
	case in.Policy == nil:
		return fmt.Errorf("%w: nil policy", model.ErrUsage)
	case in.Snapshot == nil:
		return fmt.Errorf("%w: nil snapshot", model.ErrUsage)
	case in.Commit != nil && in.Mode != model.ModeCommit:
		return fmt.Errorf("%w: a commit in %s mode", model.ErrUsage, in.Mode)
	case in.Mode == model.ModeHook:
		if err := in.Event.Validate(); err != nil {
			return fmt.Errorf("dispatch: %w", err)
		}
		return nil
	case in.Event != nil:
		return fmt.Errorf("%w: an event in %s mode", model.ErrUsage, in.Mode)
	case in.Mode == model.ModeCommit:
		return validateCommit(in.Commit)
	default:
		return nil
	}
}

// resolveOptions lays the policy values over the schema defaults.
func resolveOptions(spec model.Spec, policy *model.EffectivePolicy) model.Options {
	options := spec.Options.Defaults()
	maps.Copy(options, policy.OptionsFor(spec.ID))
	return options
}

// runOne calls the check and turns a panic or an error into Unmeasured, so
// a crashing rule denies instead of failing open on an agent that treats a
// crash as success. An error keeps the findings the check answers beside
// it, those it established before it stopped, so a deadline that stops a
// check between its targets leaves them to the decision; a panic keeps
// none.
func runOne(ctx context.Context, c model.Check, in model.Input) model.Result {
	var res model.Result
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				res, err = model.Result{Status: model.Unmeasured(fmt.Sprintf("panic: %v", r))}, nil
			}
		}()
		res, err = c.Run(ctx, in)
	}()
	if err != nil {
		return model.Result{Status: model.Unmeasured(err.Error()), Scope: res.Scope, Findings: res.Findings}
	}
	return res
}

// normalize fills what a check may leave empty, applies the committed
// exceptions and orders the findings, so every consumer sees one shape.
func normalize(spec model.Spec, policy *model.EffectivePolicy, res model.Result) model.Result {
	if !res.Status.Kind.Valid() {
		res.Status = model.Unmeasured("the check returned no status")
	}
	for i := range res.Findings {
		f := &res.Findings[i]
		if f.Check == "" {
			f.Check = spec.ID
		}
		if f.Rule == "" {
			f.Rule = spec.Rule
		}
		if !f.Severity.Valid() {
			f.Severity = model.SeverityFail
		}
		if f.Fingerprint == "" {
			f.Fingerprint = model.Fingerprint(*f)
		}
		if record, ok := policy.Excepted(f.Check, f.Path); ok {
			f.Severity = model.SeverityNote
			f.Message += " (excepted by " + record + ")"
		}
	}
	model.SortFindings(res.Findings)
	return res
}

// validateCommit refuses a commit mode without its commit or with one git
// never makes, the way an event is validated in hook mode.
func validateCommit(commit *model.Commit) error {
	if commit == nil {
		return fmt.Errorf("%w: commit mode without a commit", model.ErrUsage)
	}
	if err := commit.Validate(); err != nil {
		return fmt.Errorf("dispatch: %w", err)
	}
	return nil
}
