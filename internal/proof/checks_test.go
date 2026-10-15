package proof_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

// dashCheck reports an em dash on every target line that holds one.
type dashCheck struct {
	id       string
	requires []string
}

func (d dashCheck) Spec() model.Spec {
	return model.Spec{ID: d.id, Rule: "rules/3@1", Scope: model.ScopeFile, Owner: model.OwnerCore, Proof: d.id, Requires: d.requires}
}

func (dashCheck) Run(_ context.Context, in model.Input) (model.Result, error) {
	res := model.Result{Status: model.Measured(), Scope: fmt.Sprintf("%d files", len(in.Targets))}
	for _, target := range in.Targets {
		data, err := in.Snapshot.Read(target.Path)
		if err != nil {
			return model.Result{Status: model.Unmeasured(err.Error())}, nil
		}
		for i, line := range strings.Split(string(data), "\n") {
			if strings.ContainsRune(line, emDash) {
				res.Findings = append(res.Findings, model.Finding{Code: "em-dash", Path: target.Path, Line: i + 1, Message: "em dash in prose", Severity: model.SeverityFail})
			}
		}
	}
	return res, nil
}

// noVerifyCheck denies a shell command that skips the git hooks.
type noVerifyCheck struct{}

func (noVerifyCheck) Spec() model.Spec {
	return model.Spec{ID: "shell.no-verify", Rule: "rules/12@1", Scope: model.ScopeEvent, Owner: model.OwnerCore, Proof: "shell.no-verify"}
}

func (noVerifyCheck) Run(_ context.Context, in model.Input) (model.Result, error) {
	if in.Event.Shape != model.ShapeShell {
		return model.Result{Status: model.NotApplicable("not a shell event")}, nil
	}
	res := model.Result{Status: model.Measured(), Scope: "1 command"}
	if strings.Contains(in.Event.Shell.Command, "--no-verify") {
		res.Findings = []model.Finding{{Code: "no-verify", Message: "the command skips the hooks", Severity: model.SeverityDeny}}
	}
	return res, nil
}

func registry(t *testing.T, checks ...model.Check) *engine.Registry {
	t.Helper()
	b := engine.New()
	for _, c := range checks {
		if err := b.Register(c); err != nil {
			t.Fatal(err)
		}
	}
	reg, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	return reg
}
