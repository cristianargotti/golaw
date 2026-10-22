// Package hooktest judges the payloads of an agent adapter through the
// hook core in tests: an enrolled repository in memory under the proof
// policy, the production registry, the overlay of the source package and a
// state directory of the test, and the proof passes over a manifest of the
// adapter's routes. Only tests import it.
package hooktest

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/checks"
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
	"github.com/cristianargotti/golaw/internal/source"
)

// Root is the repository root every payload of a corpus spells; Payload
// replaces it with a real root before the payload is judged.
const Root = "/project"

// stamp is the clock of the state store: a fixed instant, so no test
// helper reads the wall clock.
func stamp() time.Time { return time.Date(2026, time.October, 21, 12, 0, 0, 0, time.UTC) }

// Payload is a payload of a corpus with every spelling of Root replaced by
// root, in slashes, so its paths name the real repository.
func Payload(data []byte, root string) []byte {
	return bytes.ReplaceAll(data, []byte(Root), []byte(filepath.ToSlash(root)))
}

// Repository is a real, empty directory for the root of a repository whose
// files live in memory, named by its real path as discovery names a root.
func Repository(tb testing.TB) string {
	tb.Helper()
	root, err := filepath.EvalSymlinks(tb.TempDir())
	if err != nil {
		tb.Fatal(err)
	}
	return root
}

// Dependencies binds the hook core to an enrolled repository at root that
// holds files, judged by the production registry under the proof policy,
// with its state in a directory of the test.
func Dependencies(tb testing.TB, root string, files map[string][]byte) hook.Dependencies {
	tb.Helper()
	registry, err := checks.Registry()
	if err != nil {
		tb.Fatal(err)
	}
	return bind(registry, root, tb.TempDir(), files)
}

// Evaluate judges a payload of a corpus through the hook core with the
// adapter of an agent, over a repository in memory that holds files.
func Evaluate(tb testing.TB, agent string, adapter hook.Adapter, payload []byte, files map[string][]byte) (model.Event, hook.Result) {
	tb.Helper()
	root := Repository(tb)
	return hook.Evaluate(tb.Context(), agent, Payload(payload, root), adapter, Dependencies(tb, root, files))
}

// Answer is the reply the adapter of an agent gives a payload of a corpus,
// through the whole hook run with its deadline, over a repository in
// memory that holds files.
func Answer(tb testing.TB, agent string, adapter hook.Adapter, payload []byte, files map[string][]byte) hook.Reply {
	tb.Helper()
	root := Repository(tb)
	return hook.Run(tb.Context(), agent, bytes.NewReader(Payload(payload, root)), adapter, Dependencies(tb, root, files))
}

func bind(registry *engine.Registry, root, state string, files map[string][]byte) hook.Dependencies {
	tree := proof.NewTree(files)
	repository := hook.Repository{
		Root: root, Identity: root,
		Enrollment: model.Enrollment{State: model.EnrollmentEnrolled, PolicyDigest: "proof"},
	}
	return hook.Dependencies{
		Registry: registry, State: hook.Store{Dir: state, Now: stamp()},
		Enroll: func(context.Context, string, string) (hook.Repository, error) { return repository, nil },
		Policy: func(context.Context, hook.Repository) (*model.EffectivePolicy, error) {
			return proof.Policy(), nil
		},
		Snapshot: func(context.Context, hook.Repository) (model.Snapshot, error) { return tree, nil },
		Overlay:  overlay,
	}
}

func overlay(_ context.Context, base model.Snapshot, candidates []model.Candidate, content map[string][]byte) (model.Snapshot, error) {
	o, _, err := source.NewOverlay(base, source.FromCandidates(candidates, content))
	if err != nil {
		return nil, fmt.Errorf("fold the candidates: %w", err)
	}
	return o, nil
}
