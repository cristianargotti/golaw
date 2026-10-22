package hooktest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks"
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// The files of a route fixture: the payload under proof, an optional pre
// event judged first, and the directory of the repository at that pre
// event; every other file is the repository the payload is judged over.
const (
	PayloadFile = "payload.json"
	PreFile     = "pre.json"
	BeforeDir   = "before/"
)

// routeRule is the rule reference of every adapter route: the per agent
// table of section 8.2.
const routeRule = "rules/8.2@1"

// route is one route of an adapter as a check of the proof: it judges the
// payload of a fixture through the hook core and yields its code when the
// hook does not allow it. Every route judges every fixture, so each route
// carries a code of its own.
type route struct {
	id, code, agent string
	adapter         hook.Adapter
	registry        *engine.Registry
	root, state     string
}

func (r route) Spec() model.Spec {
	return model.Spec{ID: r.id, Rule: routeRule, Scope: model.ScopeTree, Owner: model.OwnerCore, Proof: r.id}
}

func (r route) Run(ctx context.Context, in model.Input) (model.Result, error) {
	payload, pre, before, after, err := split(in.Snapshot)
	if err != nil {
		return model.Result{}, err
	}
	state, err := os.MkdirTemp(r.state, "run")
	if err != nil {
		return model.Result{}, fmt.Errorf("state of %s: %w", r.id, err)
	}
	if pre != nil {
		hook.Evaluate(ctx, r.agent, Payload(pre, r.root), r.adapter, bind(r.registry, r.root, state, before))
	}
	_, result := hook.Evaluate(ctx, r.agent, Payload(payload, r.root), r.adapter, bind(r.registry, r.root, state, after))
	out := model.Result{Status: model.Measured(), Scope: "1 payload"}
	if result.Outcome != hook.Allow {
		out.Findings = []model.Finding{{Code: r.code, Message: result.Reason, Severity: model.SeverityNote}}
	}
	return out, nil
}

// split reads a fixture: the payload, the pre event when there is one, the
// repository at the pre event and the repository the payload is judged
// over.
func split(snapshot model.Snapshot) ([]byte, []byte, map[string][]byte, map[string][]byte, error) {
	var payload, pre []byte
	before, after := map[string][]byte{}, map[string][]byte{}
	for _, f := range snapshot.Files() {
		data, err := snapshot.Read(f.Path)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("read %s: %w", f.Path, err)
		}
		switch {
		case f.Path == PayloadFile:
			payload = data
		case f.Path == PreFile:
			pre = data
		case strings.HasPrefix(f.Path, BeforeDir):
			before[strings.TrimPrefix(f.Path, BeforeDir)] = data
		default:
			after[f.Path] = data
		}
	}
	if payload == nil {
		return nil, nil, nil, nil, fmt.Errorf("%w: the fixture holds no %s", proof.ErrFixture, PayloadFile)
	}
	return payload, pre, before, after, nil
}

// Prove runs the proof of an adapter's routes over the manifest at path:
// each entry a route of the agent whose fixtures are a payload.json, an
// optional pre.json judged first over the files under before/, and the
// files of the repository. The baseline, deletion, over-fire and
// capability passes of internal/proof run over them.
func Prove(t *testing.T, manifest, agent string, adapter hook.Adapter) {
	t.Helper()
	m, err := proof.LoadManifests(manifest)
	if err != nil {
		t.Fatal(err)
	}
	production, err := checks.Registry()
	if err != nil {
		t.Fatal(err)
	}
	builder, root, state := engine.New(), Repository(t), t.TempDir()
	for _, e := range m.Checks {
		if registerErr := builder.Register(route{id: e.ID, code: e.Code, agent: agent, adapter: adapter, registry: production, root: root, state: state}); registerErr != nil {
			t.Fatal(registerErr)
		}
	}
	registry, err := builder.Build(engine.WithProofs(m.Names()))
	if err != nil {
		t.Fatal(err)
	}
	for _, failure := range proof.Verify(t.Context(), registry, filepath.Dir(manifest), m) {
		t.Error(failure.String())
	}
}
