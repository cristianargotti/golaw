package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestSuggestionLineNamesTheEvidenceAndTheCommand(t *testing.T) {
	t.Parallel()
	suggestions := []bootstrap.Suggestion{{Capability: "api", Evidence: "api/swagger.yaml"}, {Capability: "image", Evidence: "Dockerfile"}}
	for _, tc := range []struct{ language, want string }{
		{"pt-BR", "Capacidades que os arquivos sugerem, nenhuma habilitada sem pedido: api (api/swagger.yaml), image (Dockerfile). Para habilitá-las: golaw init --capabilities api,go,image"},
		{"en", "Capabilities the files suggest, none enabled unless asked: api (api/swagger.yaml), image (Dockerfile). To enable them: golaw init --capabilities api,go,image"},
	} {
		if got := suggestionLine(tc.language, []string{"go"}, suggestions); got != tc.want {
			t.Fatalf("%s line = %q", tc.language, got)
		}
	}
	if got := suggestionLine("pt-BR", []string{"go"}, nil); got != "" {
		t.Fatalf("no suggestion printed %q", got)
	}
}

func TestInitPrintsSuggestionsAndEnablesNone(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	repo.Commit("chore: establish application", map[string]string{
		"go.mod": "module example.test/birth\n\ngo 1.27\n", "Dockerfile": "FROM scratch\n",
		"api/swagger.yaml": "swagger: \"2.0\"\n",
	})
	ta := initBindApp(t)
	if code := ta.run(t.Context(), []string{"init", repo.Root}); code != exitClean {
		t.Fatalf("dry run exit = %d: %s", code, ta.stderr.String())
	}
	out := ta.stdout.String()
	if !strings.Contains(out, "api (api/swagger.yaml), image (Dockerfile)") || !strings.Contains(out, "+capabilities: [go]") {
		t.Fatalf("dry run = %s", out)
	}
}

func TestSuggestionCommandKeepsTheEnabledSetUnchanged(t *testing.T) {
	t.Parallel()
	enabled := make([]string, 3, 8)
	copy(enabled, []string{"image", "go", "go"})
	before := slices.Clone(enabled[:cap(enabled)])
	suggestions := []bootstrap.Suggestion{
		{Capability: "api", Evidence: "api/swagger.yaml"},
		{Capability: "image", Evidence: "Dockerfile"},
	}
	got := suggestionLine("en-GB", enabled, suggestions)
	if !strings.HasSuffix(got, "golaw init --capabilities api,go,image") {
		t.Fatalf("suggested command = %q", got)
	}
	if !slices.Equal(enabled[:cap(enabled)], before) {
		t.Fatalf("enabled storage changed: %v; want %v", enabled[:cap(enabled)], before)
	}
}

func TestInitAppliesSuggestionsWithoutEnablingCapabilitiesOrTemplates(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	repo.Commit("chore: establish application", map[string]string{
		"go.mod": "module example.test/birth\n\ngo 1.27\n", "Dockerfile": "FROM scratch\n",
		"api/swagger.yaml": "swagger: \"2.0\"\n", "db/migrations/0001_init.sql": "SELECT 1;\n",
	})
	ta := initBindApp(t)
	out := capabilityRun(t, ta, exitClean, "init", "--apply", repo.Root)
	if !strings.Contains(out, "api (api/swagger.yaml), db (db/migrations/0001_init.sql), image (Dockerfile)") ||
		!strings.Contains(string(currentPolicy(t, repo.Root)), "\ncapabilities: [go]\n") {
		t.Fatalf("apply = %s", out)
	}
	for _, name := range []string{"redocly.yaml", "api/codegen/server.yaml", "sqlc.yaml", "sqlc.vet.yaml", ".squawk.toml"} {
		if _, err := os.Stat(filepath.Join(repo.Root, filepath.FromSlash(name))); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("suggestion wrote %s: %v", name, err)
		}
	}
	requireUnarmed(t, repo.GitDir())
}

func TestInitSuggestionPropagatesOutputFailure(t *testing.T) {
	t.Parallel()
	compiled := model.CoreDefaults()
	plan := initPlan{
		policy:      &compiled,
		suggestions: []bootstrap.Suggestion{{Capability: "image", Evidence: "Dockerfile"}},
	}
	if err := printInitPolicy(coverageWriteFailure{}, plan); !errors.Is(err, errBoom) {
		t.Fatalf("suggestion output error = %v", err)
	}
}
