package engine_test

import (
	"context"
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

func decide(t *testing.T, in model.Input, checks ...model.Check) engine.Decision {
	t.Helper()
	run, err := engine.Dispatch(t.Context(), build(t, checks...), in)
	if err != nil {
		t.Fatal(err)
	}
	return engine.Decide(run)
}

func TestDecideClean(t *testing.T) {
	t.Parallel()
	d := decide(t, scanInput(), fake{spec: spec("text.dashes", model.ScopeFile)})
	if d.Verdict != engine.VerdictClean || d.Reason != "" || len(d.Findings) != 0 || len(d.Unmeasured) != 0 || d.Mode != model.ModeScan {
		t.Errorf("Decide() = %+v", d)
	}
}

func TestDecideFailsScanAndDeniesHook(t *testing.T) {
	t.Parallel()
	check := findingCheck("text.dashes", finding("em-dash", "docs/a.md", 3))
	d := decide(t, scanInput(), check)
	if d.Verdict != engine.VerdictFail || d.Reason != "text.dashes: found em-dash (docs/a.md:3)" {
		t.Errorf("scan: %+v", d)
	}
	d = decide(t, hookInput(model.ShapeWrite), check)
	if d.Verdict != engine.VerdictDeny || d.Mode != model.ModeHook {
		t.Errorf("hook: %+v", d)
	}
}

func TestDecideReasonFormats(t *testing.T) {
	t.Parallel()
	noLine := finding("em-dash", "docs/a.md", 0)
	noPath := model.Finding{Code: "no-verify", Message: "git commit skips the hooks", Severity: model.SeverityDeny}
	d := decide(t, scanInput(), findingCheck("text.dashes", noLine))
	if d.Reason != "text.dashes: found em-dash (docs/a.md)" {
		t.Errorf("no line: %q", d.Reason)
	}
	d = decide(t, scanInput(), findingCheck("shell.no-verify", noPath))
	if d.Reason != "shell.no-verify: git commit skips the hooks" {
		t.Errorf("no path: %q", d.Reason)
	}
}

func TestDecideNotesNeverCount(t *testing.T) {
	t.Parallel()
	note := finding("em-dash", "docs/a.md", 3)
	note.Severity = model.SeverityNote
	d := decide(t, scanInput(), findingCheck("text.dashes", note))
	if d.Verdict != engine.VerdictClean || len(d.Findings) != 1 || d.Reason != "" {
		t.Errorf("notes only: %+v", d)
	}
}

func TestDecideUnmeasuredFails(t *testing.T) {
	t.Parallel()
	broken := fake{spec: spec("policy.chart", model.ScopeTree), run: func(context.Context, model.Input) (model.Result, error) {
		return model.Result{Status: model.Unmeasured("values file uses an unsupported syntax")}, nil
	}}
	d := decide(t, scanInput(), broken)
	if d.Verdict != engine.VerdictFail || len(d.Unmeasured) != 1 || d.Unmeasured[0].Check != "policy.chart" {
		t.Errorf("unmeasured: %+v", d)
	}
	if d.Reason != "policy.chart: not measured: values file uses an unsupported syntax" {
		t.Errorf("reason: %q", d.Reason)
	}
	d = decide(t, hookInput(model.ShapeShell), fake{spec: spec("shell.push", model.ScopeEvent), run: broken.run})
	if d.Verdict != engine.VerdictDeny {
		t.Errorf("unmeasured in a hook must deny: %+v", d)
	}
	withFinding := decide(t, scanInput(), broken, findingCheck("text.dashes", finding("em-dash", "a.md", 1)))
	if withFinding.Reason != "text.dashes: found em-dash (a.md:1)" {
		t.Errorf("a finding wins the reason over an unmeasured check: %q", withFinding.Reason)
	}
}

func TestVerdictNames(t *testing.T) {
	t.Parallel()
	names := map[engine.Verdict]string{
		engine.VerdictClean: "clean", engine.VerdictFail: "fail", engine.VerdictDeny: "deny", engine.Verdict(0): "unknown",
	}
	for v, want := range names {
		if v.String() != want {
			t.Errorf("Verdict(%d).String() = %q, want %q", int(v), v.String(), want)
		}
	}
}

// TestDecideCountsAFileLeftUnreadByMode holds a check that went on past a
// file over the read bound to the mode: a scan counts it unmeasured, never
// clean, while a hook keeps its findings and names the file unread for
// its notice.
func TestDecideCountsAFileLeftUnreadByMode(t *testing.T) {
	t.Parallel()
	partial := func(findings ...model.Finding) fake {
		return fake{spec: spec("text.dashes", model.ScopeFile), run: func(context.Context, model.Input) (model.Result, error) {
			return model.Result{Status: model.Measured(), Findings: findings, Unread: []string{"big.md", "big.md"}}, nil
		}}
	}
	d := decide(t, scanInput(), partial())
	if d.Verdict != engine.VerdictFail || len(d.Unmeasured) != 1 || d.Reason != "text.dashes: not measured: read big.md: file larger than the read bound" {
		t.Errorf("scan: %+v", d)
	}
	d = decide(t, hookInput(model.ShapePost), partial())
	if d.Verdict != engine.VerdictClean || len(d.Unmeasured) != 0 || len(d.Unread) != 1 || d.Unread[0] != "big.md" {
		t.Errorf("hook without a finding: %+v", d)
	}
	d = decide(t, hookInput(model.ShapePost), partial(finding("em-dash", "a.md", 1)))
	if d.Verdict != engine.VerdictDeny || len(d.Unread) != 1 {
		t.Errorf("hook with a finding: %+v", d)
	}
}
