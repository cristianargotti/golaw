package cicheck_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

type auditRule struct {
	id      string
	rule    string
	match   func(string) bool
	inspect func(string, []byte) string
}

// Spec binds each repository assertion to its independent proof.
func (r auditRule) Spec() model.Spec {
	return model.Spec{
		ID: r.id, Rule: r.rule, Scope: model.ScopeFile,
		Owner: model.OwnerCore, Proof: r.id,
	}
}

// Run reads the supplied snapshot so fixtures exercise the real assertion.
func (r auditRule) Run(_ context.Context, in model.Input) (model.Result, error) {
	result := model.Result{Status: model.Measured(), Scope: r.id}
	for _, file := range in.Targets {
		if !r.match(file.Path) {
			continue
		}
		data, err := in.Snapshot.Read(file.Path)
		if err != nil {
			result.Status = model.Unmeasured(err.Error())
			return result, nil
		}
		if reason := r.inspect(file.Path, data); reason != "" {
			result.Findings = append(result.Findings, model.Finding{
				Path: file.Path, Code: strings.ReplaceAll(r.id, ".", "-"),
				Message: reason, Severity: model.SeverityFail,
			})
		}
	}
	return result, nil
}

func auditRegistry(t *testing.T, manifest proof.Manifest) *engine.Registry {
	t.Helper()
	builder := engine.New()
	for _, rule := range auditRules(t) {
		if err := builder.Register(rule); err != nil {
			t.Fatal(err)
		}
	}
	registry, err := builder.Build(engine.WithProofs(manifest.Names()))
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func kitBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func kitTree(t *testing.T) *proof.Tree {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", "ls-files", "-z", "-co", "--exclude-standard")
	command.Dir = filepath.Join("..", "..")
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string][]byte)
	for path := range strings.SplitSeq(string(output), "\x00") {
		if path == "" {
			continue
		}
		for _, rule := range auditRules(t) {
			if rule.match(path) {
				files[path] = kitBytes(t, path)
				break
			}
		}
	}
	return proof.NewTree(files)
}

func requireText(t *testing.T, path string, fragments ...string) { //nolint:unused // This helper is reserved for document content assertions.
	t.Helper()
	text := string(kitBytes(t, path))
	for _, fragment := range fragments {
		if !strings.Contains(text, fragment) {
			t.Errorf("%s lacks %q", path, fragment)
		}
	}
}
