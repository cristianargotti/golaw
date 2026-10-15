package engine_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

func finding(code, path string, line int) model.Finding {
	return model.Finding{Code: code, Path: path, Line: line, Message: "found " + code, Severity: model.SeverityFail}
}

func findingCheck(id string, findings ...model.Finding) fake {
	return fake{spec: spec(id, model.ScopeFile), run: func(context.Context, model.Input) (model.Result, error) {
		return model.Result{Status: model.Measured(), Scope: "1 file", Findings: findings}, nil
	}}
}

func TestDispatchRefusesBadInput(t *testing.T) {
	t.Parallel()
	reg := build(t, fake{spec: spec("text.dashes", model.ScopeFile)})
	cases := map[string]func() (*engine.Registry, model.Input){
		"nil registry": func() (*engine.Registry, model.Input) { return nil, scanInput() },
		"invalid mode": func() (*engine.Registry, model.Input) { in := scanInput(); in.Mode = 0; return reg, in },
		"nil policy":   func() (*engine.Registry, model.Input) { in := scanInput(); in.Policy = nil; return reg, in },
		"nil snapshot": func() (*engine.Registry, model.Input) { in := scanInput(); in.Snapshot = nil; return reg, in },
		"hook no event": func() (*engine.Registry, model.Input) {
			in := hookInput(model.ShapeWrite)
			in.Event = nil
			return reg, in
		},
		"hook bad event": func() (*engine.Registry, model.Input) {
			in := hookInput(model.ShapeWrite)
			in.Event.Agent = "Bad Agent"
			return reg, in
		},
		"scan with event": func() (*engine.Registry, model.Input) {
			in := scanInput()
			in.Event = hookInput(model.ShapeRead).Event
			return reg, in
		},
	}
	for name, make := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r, in := make()
			if _, err := engine.Dispatch(t.Context(), r, in); !errors.Is(err, model.ErrUsage) {
				t.Fatalf("Dispatch() error = %v, want ErrUsage", err)
			}
		})
	}
}

func TestDispatchFillsAndOrdersFindings(t *testing.T) {
	t.Parallel()
	reg := build(t, findingCheck("text.dashes", finding("em-dash", "b.md", 2), finding("em-dash", "a.md", 9)))
	run, err := engine.Dispatch(t.Context(), reg, scanInput())
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Outcomes) != 1 || run.Outcomes[0].Spec.ID != "text.dashes" || run.Outcomes[0].Result.Scope != "1 file" {
		t.Fatalf("outcomes = %+v", run.Outcomes)
	}
	findings := run.Findings()
	if len(findings) != 2 || findings[0].Path != "a.md" || findings[1].Path != "b.md" {
		t.Fatalf("findings = %+v", findings)
	}
	f := findings[0]
	if f.Check != "text.dashes" || f.Rule != "rules/1@1" || f.Fingerprint != model.Fingerprint(f) || f.Severity != model.SeverityFail {
		t.Errorf("finding not normalized: %+v", f)
	}
	if run.Mode != model.ModeScan || len(run.Unmeasured()) != 0 {
		t.Errorf("run = %+v", run)
	}
}

func TestDispatchDefaultsSeverityAndKeepsFilled(t *testing.T) {
	t.Parallel()
	given := model.Finding{Check: "other.id", Rule: "rules/9@9", Code: "c", Path: "a.go", Message: "m", Fingerprint: "given"}
	reg := build(t, findingCheck("text.dashes", given))
	run, err := engine.Dispatch(t.Context(), reg, scanInput())
	if err != nil {
		t.Fatal(err)
	}
	got := run.Findings()[0]
	if got.Check != "other.id" || got.Rule != "rules/9@9" || got.Fingerprint != "given" || got.Severity != model.SeverityFail {
		t.Errorf("filled fields must be kept and the zero severity must become fail: %+v", got)
	}
}

func TestDispatchAppliesExceptions(t *testing.T) {
	t.Parallel()
	reg := build(t,
		findingCheck("goast.secret-fields", finding("bare-secret", "internal/auth/session.go", 4)),
		findingCheck("goast.wall-clock", finding("wall-clock", "internal/auth/session.go", 9)),
	)
	in := scanInput()
	policy := *in.Policy
	policy.Exceptions = []model.Exception{{Check: "goast.secret-fields", Path: "internal/auth/session.go", Record: "docs/adr/0009-session-token.md"}}
	in.Policy = &policy
	run, err := engine.Dispatch(t.Context(), reg, in)
	if err != nil {
		t.Fatal(err)
	}
	findings := run.Findings()
	if len(findings) != 2 {
		t.Fatalf("findings = %+v", findings)
	}
	f := findings[0]
	if f.Severity != model.SeverityNote || !strings.HasSuffix(f.Message, "(excepted by docs/adr/0009-session-token.md)") {
		t.Errorf("excepted finding = %+v", f)
	}
	if f.Fingerprint != model.Fingerprint(model.Finding{Check: f.Check, Rule: f.Rule, Path: f.Path, Code: f.Code, Message: "found bare-secret"}) {
		t.Error("the fingerprint is computed before the exception note")
	}
	// The exception names one check, so the other check on the same path
	// still fails the run.
	if other := findings[1]; other.Check != "goast.wall-clock" || other.Severity != model.SeverityFail || !other.Enforced() || other.Message != "found wall-clock" {
		t.Errorf("another check on the excepted path = %+v", other)
	}
}

var errBoom = errors.New("boom")

func TestDispatchTurnsFailuresIntoUnmeasured(t *testing.T) {
	t.Parallel()
	checks := []model.Check{
		fake{spec: spec("a.panics", model.ScopeFile), run: func(context.Context, model.Input) (model.Result, error) { panic("kaboom") }},
		fake{spec: spec("b.errs", model.ScopeFile), run: func(context.Context, model.Input) (model.Result, error) {
			return model.Result{Scope: "partial"}, errBoom
		}},
		fake{spec: spec("c.silent", model.ScopeFile), run: func(context.Context, model.Input) (model.Result, error) {
			return model.Result{}, nil
		}},
	}
	run, err := engine.Dispatch(t.Context(), build(t, checks...), scanInput())
	if err != nil {
		t.Fatal(err)
	}
	reasons := map[string]string{"a.panics": "panic: kaboom", "b.errs": "boom", "c.silent": "the check returned no status"}
	for _, o := range run.Outcomes {
		if o.Result.Status.Kind != model.StatusUnmeasured || o.Result.Status.Reason != reasons[o.Spec.ID] {
			t.Errorf("%s: %+v", o.Spec.ID, o.Result.Status)
		}
	}
	if len(run.Unmeasured()) != 3 {
		t.Errorf("Unmeasured() = %d", len(run.Unmeasured()))
	}
}

func TestDispatchHonorsDeadline(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	run, err := engine.Dispatch(ctx, build(t, findingCheck("text.dashes", finding("em-dash", "a.md", 1))), scanInput())
	if err != nil {
		t.Fatal(err)
	}
	status := run.Outcomes[0].Result.Status
	if status.Kind != model.StatusUnmeasured || !strings.HasPrefix(status.Reason, "deadline: ") {
		t.Errorf("status after cancel = %+v", status)
	}
}

func TestDispatchResolvesOptionsAndRecordsNotApplicable(t *testing.T) {
	t.Parallel()
	s := spec("goast.file-length", model.ScopeFile)
	s.Options = model.OptionSchema{
		{Name: "limit", Kind: model.OptionInt, Tighten: model.TightenDown, Default: model.IntValue(300)},
		{Name: "tests", Kind: model.OptionBool, Tighten: model.TightenNone, Default: model.BoolValue(true)},
	}
	var seen model.Options
	sized := fake{spec: s, run: func(_ context.Context, in model.Input) (model.Result, error) {
		seen = in.Options
		return model.Result{Status: model.Measured()}, nil
	}}
	event := spec("shell.no-verify", model.ScopeEvent)
	in := scanInput()
	policy := *in.Policy
	policy.Options = map[string]model.Options{"goast.file-length": {"limit": model.IntValue(200)}}
	in.Policy = &policy
	run, err := engine.Dispatch(t.Context(), build(t, sized, fake{spec: event}), in)
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := seen.Int("limit"); v != 200 {
		t.Errorf("policy option must override the default: %v", seen)
	}
	if v, _ := seen.Bool("tests"); !v {
		t.Errorf("schema default must be present: %v", seen)
	}
	if o := run.Outcomes[1]; o.Spec.ID != "shell.no-verify" || o.Result.Status.Kind != model.StatusNotApplicable {
		t.Errorf("event check in scan mode = %+v", o)
	}
}
