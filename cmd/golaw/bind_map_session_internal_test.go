package main

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestTheSessionBindingAnswersTheMapAtItsBudget(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	h := hookBinding{binding: b}
	repo := gentest.Module(t, gentest.Contracts())
	text := h.session(t.Context(), hook.Repository{Root: repo.Root}, proof.Policy())
	if !strings.HasPrefix(text, "golaw map layout: ") || !strings.Contains(text, "port growth.Ledger ") || len(text) > 8000 {
		t.Fatalf("text =\n%s", text)
	}
	files := gentest.Contracts()
	files[manifest.Path] = []byte("golaw-gen: 9\n")
	broken := gentest.Module(t, files)
	if got := h.session(t.Context(), hook.Repository{Root: broken.Root}, proof.Policy()); got != "" {
		t.Fatalf("a broken manifest answered %q", got)
	}
}

func TestHookDependenciesBindTheSession(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	deps, err := b.hookDependencies(agentUnderTest(), now())
	if err != nil || deps.Session == nil {
		t.Fatalf("deps.Session bound = %t, %v", deps.Session != nil, err)
	}
}
