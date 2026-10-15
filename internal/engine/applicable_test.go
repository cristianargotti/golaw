package engine_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

// emptySnapshot is a universe with no files.
type emptySnapshot struct{}

func (emptySnapshot) Files() []model.File { return nil }

func (emptySnapshot) Read(string) ([]byte, error) { return nil, model.ErrNotInSnapshot }

func (emptySnapshot) Class(string) model.FileClass { return model.ClassUnknown }

func scanInput() model.Input {
	policy := model.CoreDefaults()
	policy.Capabilities = []string{"go"}
	return model.Input{Mode: model.ModeScan, Policy: &policy, Snapshot: emptySnapshot{}}
}

func hookInput(shape model.EventShape) model.Input {
	in := scanInput()
	in.Mode = model.ModeHook
	e := &model.Event{Shape: shape, Agent: "tester", Tool: "Write"}
	switch shape {
	case model.ShapeWrite:
		e.Candidates = []model.Candidate{{Path: "a.go", Op: model.CandidateCreate, Reconstructed: true}}
	case model.ShapeRead:
		e.Read = &model.ReadRequest{Path: "a.go"}
	case model.ShapeShell:
		e.Shell = &model.ShellCommand{Command: "git status"}
	case model.ShapePost:
		e.Post = &model.PostResult{Response: "ok"}
	default:
	}
	in.Event = e
	return in
}

func TestApplicabilityByScope(t *testing.T) {
	t.Parallel()
	inputs := map[string]model.Input{
		"scan": scanInput(), "staged": func() model.Input { in := scanInput(); in.Mode = model.ModeStaged; return in }(),
		"commit":     func() model.Input { in := scanInput(); in.Mode = model.ModeCommit; return in }(),
		"hook write": hookInput(model.ShapeWrite), "hook read": hookInput(model.ShapeRead),
		"hook shell": hookInput(model.ShapeShell), "hook post": hookInput(model.ShapePost),
	}
	applies := map[model.Scope][]string{
		model.ScopeFile:   {"scan", "staged", "hook write", "hook post"},
		model.ScopeTree:   {"scan", "staged"},
		model.ScopeEvent:  {"hook write", "hook read", "hook shell", "hook post"},
		model.ScopeCommit: {"commit"},
	}
	for scope, modes := range applies {
		for name, in := range inputs {
			want := false
			for _, m := range modes {
				want = want || m == name
			}
			status, ok := engine.Applicability(spec("x.y", scope), in)
			if ok != want {
				t.Errorf("%s in %s: ok=%v, want %v (%s)", scope, name, ok, want, status.Reason)
			}
			if !ok && (status.Kind != model.StatusNotApplicable || !strings.Contains(status.Reason, "scope")) {
				t.Errorf("%s in %s: status %+v", scope, name, status)
			}
		}
	}
}

func TestApplicabilityByOwnerAndCapability(t *testing.T) {
	t.Parallel()
	in := scanInput()
	profile := spec("policy.chart", model.ScopeTree)
	profile.Owner = "acme"
	if status, ok := engine.Applicability(profile, in); ok || !strings.Contains(status.Reason, "profile acme") {
		t.Errorf("profile check without the profile: %v %+v", ok, status)
	}
	policy := *in.Policy
	policy.Profile = "acme"
	in.Policy = &policy
	if _, ok := engine.Applicability(profile, in); !ok {
		t.Error("profile check under its profile must apply")
	}
	needs := spec("policy.openapi", model.ScopeTree)
	needs.Needs = []string{"go", "api"}
	if status, ok := engine.Applicability(needs, in); ok || !strings.Contains(status.Reason, "capability api") {
		t.Errorf("missing capability: %v %+v", ok, status)
	}
	policy.Capabilities = []string{"api", "go"}
	if _, ok := engine.Applicability(needs, in); !ok {
		t.Error("every capability enabled must apply")
	}
}

func TestApplicabilityRefusesInvalidModeAndScope(t *testing.T) {
	t.Parallel()
	in := scanInput()
	in.Mode = 0
	if _, ok := engine.Applicability(spec("x.y", model.ScopeFile), in); ok {
		t.Error("an invalid mode applies to nothing")
	}
	if _, ok := engine.Applicability(spec("x.y", 0), hookInput(model.ShapeWrite)); ok {
		t.Error("an invalid scope applies to nothing")
	}
}

func TestApplicabilityByRequirement(t *testing.T) {
	t.Parallel()
	history := spec("commit.author", model.ScopeFile)
	history.Requires = []string{"git-history"}
	if _, ok := engine.Applicability(history, scanInput()); !ok {
		t.Error("git history is provided in scan mode")
	}
	external := spec("policy.chart", model.ScopeTree)
	external.Requires = []string{"external-evidence"}
	status, ok := engine.Applicability(external, scanInput())
	if ok || status.Kind != model.StatusUnmeasured || !strings.Contains(status.Reason, "external-evidence") {
		t.Errorf("external evidence in scan mode: %v %+v", ok, status)
	}
	types := spec("goast.types", model.ScopeFile)
	types.Requires = []string{"go-types"}
	status, ok = engine.Applicability(types, hookInput(model.ShapeWrite))
	if ok || status.Kind != model.StatusNotApplicable || !strings.Contains(status.Reason, "hook mode") {
		t.Errorf("go types in hook mode: %v %+v", ok, status)
	}
	commit := scanInput()
	commit.Mode = model.ModeCommit
	historyCommit := spec("commit.author", model.ScopeCommit)
	historyCommit.Requires = []string{"git-history", "go-types"}
	if status, ok := engine.Applicability(historyCommit, commit); ok || status.Kind != model.StatusUnmeasured {
		t.Errorf("go types in commit mode: %v %+v", ok, status)
	}
}
