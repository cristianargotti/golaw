package hook_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

type probeCheck struct{}

func (probeCheck) Spec() model.Spec {
	return model.Spec{
		ID: "text.probe", Rule: "rules/3@1", Scope: model.ScopeFile,
		Owner: model.OwnerCore, Proof: "text.probe",
	}
}

func (probeCheck) Run(_ context.Context, in model.Input) (model.Result, error) {
	result := model.Result{Status: model.Measured()}
	for _, target := range in.Targets {
		content, err := in.Snapshot.Read(target.Path)
		if err != nil {
			return model.Result{}, fmt.Errorf("probe read: %w", err)
		}
		if strings.Contains(string(content), "bad") {
			result.Findings = append(result.Findings, model.Finding{
				Path: target.Path, Code: "invalid-text",
				Message: "invalid text", Severity: model.SeverityDeny,
			})
		}
	}
	return result, nil
}

func request(content string) hook.Request {
	return hook.Request{
		Event: model.Event{
			Agent: "claude", Tool: "Write", Shape: model.ShapeWrite,
			SessionID: "session", AgentID: "agent", ToolUseID: "tool",
			Candidates: []model.Candidate{{
				Path: "a.go", Op: model.CandidateReplace,
				Reconstructed: true, BaseHash: hook.Hash([]byte("safe")),
			}},
		},
		Content: map[string][]byte{"a.go": []byte(content)},
	}
}

func harness(t *testing.T, req hook.Request) (hook.Adapter, hook.Dependencies) {
	t.Helper()
	root := t.TempDir()
	req.Event.Cwd = root
	return harnessAdapter(req), harnessDependencies(t, root)
}

// harnessAdapter ignores the bytes and encodes plain text, so the hook
// tests exercise the engine path without any vendor wire format.
func harnessAdapter(req hook.Request) hook.Adapter {
	return hook.Adapter{
		Header: func([]byte) (model.Event, error) { return req.Event, nil },
		Decode: func([]byte, string, model.Snapshot) (hook.Request, error) {
			return req, nil
		},
		Encode: func(_ model.Event, result hook.Result) hook.Reply {
			code := 0
			if result.CannotJudge {
				code = 2
			}
			return hook.Reply{
				Stdout: []byte(result.Outcome), Stderr: result.Reason, ExitCode: code,
			}
		},
	}
}

// harnessDependencies start from an enrolled repository whose only file is
// safe, so each test changes exactly the dependency it is about.
func harnessDependencies(t *testing.T, root string) hook.Dependencies {
	t.Helper()
	registry := engine.New()
	if err := registry.Register(probeCheck{}); err != nil {
		t.Fatal(err)
	}
	built, err := registry.Build()
	if err != nil {
		t.Fatal(err)
	}
	policy := proof.Policy()
	repository := hook.Repository{
		Root: root, Identity: root,
		Enrollment: model.Enrollment{
			State: model.EnrollmentEnrolled, PolicyDigest: hook.Hash([]byte("raw golaw.yml")),
		},
	}
	base := proof.NewTree(map[string][]byte{"a.go": []byte("safe")})
	return hook.Dependencies{
		Registry: built, State: hook.Store{Dir: t.TempDir(), Now: time.Now()},
		Enroll: func(context.Context, string, string) (hook.Repository, error) {
			return repository, nil
		},
		Policy: func(context.Context, hook.Repository) (*model.EffectivePolicy, error) {
			return policy, nil
		},
		Snapshot: func(context.Context, hook.Repository) (model.Snapshot, error) {
			return base, nil
		},
		Overlay: func(_ context.Context, _ model.Snapshot,
			_ []model.Candidate, content map[string][]byte,
		) (model.Snapshot, error) {
			return proof.NewTree(content), nil
		},
	}
}
