package proof

import (
	"context"
	"fmt"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

// noOp keeps the spec of a check and measures nothing: the deletion pass.
type noOp struct {
	spec model.Spec
}

func (n noOp) Spec() model.Spec { return n.spec }

func (noOp) Run(context.Context, model.Input) (model.Result, error) {
	return model.Result{Status: model.Measured(), Scope: "no-op"}, nil
}

// NoOp wraps a check so that it measures nothing.
func NoOp(c model.Check) model.Check { return noOp{spec: c.Spec()} }

// always keeps the spec of a check and reports the code on every target, or
// once when there is no target: the over-fire pass.
type always struct {
	spec model.Spec
	code string
}

func (a always) Spec() model.Spec { return a.spec }

func (a always) Run(_ context.Context, in model.Input) (model.Result, error) {
	res := model.Result{Status: model.Measured(), Scope: "always"}
	if len(in.Targets) == 0 {
		res.Findings = []model.Finding{{Code: a.code, Message: "always " + a.code, Severity: model.SeverityFail}}
		return res, nil
	}
	for _, target := range in.Targets {
		res.Findings = append(res.Findings, model.Finding{Code: a.code, Path: target.Path, Line: 1, Message: "always " + a.code, Severity: model.SeverityFail})
	}
	return res, nil
}

// AlwaysFinding wraps a check so that it reports the code everywhere.
func AlwaysFinding(c model.Check, code string) model.Check {
	return always{spec: c.Spec(), code: code}
}

// Rewire builds a registry equal to reg with the check id replaced by
// wrap(check); an unknown id is a usage error.
func Rewire(reg *engine.Registry, id string, wrap func(model.Check) model.Check) (*engine.Registry, error) {
	if _, ok := reg.Lookup(id); !ok {
		return nil, fmt.Errorf("%w: rewire %s: not registered", model.ErrUsage, id)
	}
	b := engine.New()
	for _, c := range reg.Checks() {
		if c.Spec().ID == id {
			c = wrap(c)
		}
		if err := b.Register(c); err != nil {
			return nil, fmt.Errorf("rewire %s: %w", id, err)
		}
	}
	out, err := b.Build()
	if err != nil {
		return nil, fmt.Errorf("rewire %s: %w", id, err)
	}
	return out, nil
}
