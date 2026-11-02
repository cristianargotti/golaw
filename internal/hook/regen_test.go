package hook_test

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
)

const regenPorts = "internal/application/growth/ports.go"

// regenRecorder keeps the written paths each regeneration was asked for.
type regenRecorder struct {
	mu    sync.Mutex
	calls [][]string
}

func (r *regenRecorder) seen() [][]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.calls)
}

// regenDeps are the dependencies of a repository holding the contract, with
// the ladder switch on or off and a regeneration that answers report.
func regenDeps(t *testing.T, root string, on bool, rec *regenRecorder, report hook.RegenReport) hook.Dependencies {
	t.Helper()
	deps := hooktest.Dependencies(t, root, map[string][]byte{regenPorts: []byte("package growth\n")})
	load := deps.Policy
	deps.Policy = func(ctx context.Context, r hook.Repository) (*model.EffectivePolicy, error) {
		p, err := load(ctx, r)
		if err != nil {
			return nil, err
		}
		copied := *p
		copied.Ladder.HookRegenerate = on
		return &copied, nil
	}
	deps.Regenerate = func(_ context.Context, _ hook.Repository, _ *model.EffectivePolicy, written []string) hook.RegenReport {
		rec.mu.Lock()
		defer rec.mu.Unlock()
		rec.calls = append(rec.calls, written)
		return report
	}
	return deps
}

func regenReport() hook.RegenReport {
	return hook.RegenReport{
		Lines:   []string{"golaw regenerated from " + regenPorts + " (port Ledger): 3 files, 0 drift", "hole open internal/application/growth/ledgertest/cases.go:61 ledger.archive-plan declare the inputs"},
		Person:  []string{"golaw regenerou a partir de " + regenPorts + " (port Ledger): 3 arquivos, 0 com deriva", "lacuna aberta internal/application/growth/ledgertest/cases.go:61 ledger.archive-plan declare the inputs", "golaw adiou a regeneração: rode golaw gen."},
		Notices: []string{"regeneration deferred: run golaw gen"},
	}
}

func TestAPostEventOfAContractCarriesTheRegeneration(t *testing.T) {
	t.Parallel()
	root := hooktest.Repository(t)
	rec := &regenRecorder{}
	deps := regenDeps(t, root, true, rec, regenReport())
	payload := hooktest.Payload(written(t, "PostToolUse", regenPorts, "package growth\n"), root)
	_, result := hook.Evaluate(t.Context(), "claude", payload, claudeAdapter(), deps)
	if got := rec.seen(); len(got) != 1 || !slices.Equal(got[0], []string{regenPorts}) {
		t.Fatalf("calls = %v", got)
	}
	if result.Outcome != hook.Allow || len(result.Lines) < 2 || result.Lines[0] != regenReport().Lines[0] {
		t.Fatalf("result = %+v", result)
	}
	if !strings.Contains(result.Notice, "regeneration deferred: run golaw gen") || !strings.Contains(result.Human, "lacuna aberta") {
		t.Fatalf("notice %q human %q", result.Notice, result.Human)
	}
}

func TestTheRegenerationSaysItsLinesOncePerSession(t *testing.T) {
	t.Parallel()
	root := hooktest.Repository(t)
	deps := regenDeps(t, root, true, &regenRecorder{}, regenReport())
	payload := hooktest.Payload(written(t, "PostToolUse", regenPorts, "package growth\n"), root)
	_, first := hook.Evaluate(t.Context(), "claude", payload, claudeAdapter(), deps)
	_, second := hook.Evaluate(t.Context(), "claude", payload, claudeAdapter(), deps)
	if len(first.Lines) == 0 || len(second.Lines) != 0 || !strings.Contains(second.Notice, "regeneration deferred") {
		t.Fatalf("first = %+v\nsecond = %+v", first, second)
	}
}

func TestNoRegenerationBeforeTheToolOrWithTheLadderOff(t *testing.T) {
	t.Parallel()
	root := hooktest.Repository(t)
	rec := &regenRecorder{}
	pre := hooktest.Payload(written(t, "PreToolUse", regenPorts, "package growth\n"), root)
	if _, result := hook.Evaluate(t.Context(), "claude", pre, claudeAdapter(), regenDeps(t, root, true, rec, regenReport())); result.Outcome != hook.Allow {
		t.Fatalf("pre = %+v", result)
	}
	post := hooktest.Payload(written(t, "PostToolUse", regenPorts, "package growth\n"), root)
	_, off := hook.Evaluate(t.Context(), "claude", post, claudeAdapter(), regenDeps(t, root, false, rec, regenReport()))
	if len(rec.seen()) != 0 || off.Outcome != hook.Allow || len(off.Lines) != 0 {
		t.Fatalf("calls = %v, off = %+v", rec.seen(), off)
	}
	nobody := hooktest.Dependencies(t, root, map[string][]byte{regenPorts: []byte("package growth\n")})
	if _, result := hook.Evaluate(t.Context(), "claude", post, claudeAdapter(), nobody); result.Outcome != hook.Allow {
		t.Fatalf("without a binding = %+v", result)
	}
}
