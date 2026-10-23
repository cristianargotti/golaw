package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/policy"
)

// m3PreviewText is the preview of init over the birth, each mechanism named.
const m3PreviewText = "Alvos que o Makefile e golaw.mk definem ...: lint (Makefile:22), test (Makefile:28). ...\n" +
	"go.mod marca como indiretos módulos que o código importa diretamente: github.com/spf13/cobra, github.com/stretchr/testify. ...\n" +
	"+++ .editorconfig\n+[Makefile]\n+indent_style = tab\n" +
	"Capacidades que os arquivos sugerem, nenhuma habilitada sem pedido: api (api/swagger.yaml), image (Dockerfile). ...\n"

func TestM3JudgePreviewNamesEveryMechanism(t *testing.T) {
	t.Parallel()
	if err := m3JudgePreview(result{stdout: m3PreviewText}); err != nil {
		t.Fatalf("near preview = %v", err)
	}
	for _, missing := range []string{"lint (Makefile:22)", "github.com/spf13/cobra", "+[Makefile]", "image (Dockerfile)"} {
		if err := m3JudgePreview(result{stdout: strings.ReplaceAll(m3PreviewText, missing, "")}); err == nil {
			t.Fatalf("a preview without %q passed", missing)
		}
	}
	if err := m3JudgePreview(result{code: 2, stdout: m3PreviewText}); err == nil {
		t.Fatal("a failed preview passed")
	}
}

func TestM3RenameCollisionsKeepsTheTargetsUnderNewNames(t *testing.T) {
	t.Parallel()
	got := m3RenameCollisions("lint:\n\techo\n\ntest:\n\techo\n\nlint-fix:\n\techo\n")
	if got != "lint-template:\n\techo\n\ntest-compose:\n\techo\n\nlint-fix:\n\techo\n" {
		t.Fatalf("renamed = %q", got)
	}
	if !slices.Equal(m3AdoptionNames(), []string{"birth-accept", "birth-older-go", "birth-preview", "birth-scan"}) {
		t.Fatalf("names = %q", m3AdoptionNames())
	}
}

// m3WriteAccepted writes what init --apply --accept .golangci.yml leaves:
// the strict asset and a golaw.yml whose managed list records its hash.
func m3WriteAccepted(t *testing.T, dir string) {
	t.Helper()
	lint := "version: \"2\"\nlinters:\n  default: standard\n"
	data, err := bootstrap.Render(bootstrap.Inputs{
		KitVersion: m3CandidateVersion, Language: "pt-BR", Capabilities: []string{"go"},
		Layout: bootstrap.Layout{Main: bootstrap.MainRoot, Decisions: "docs/adr", Go: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err = bootstrap.ManagedPolicy(data, m3CandidateVersion, []policy.ManagedFile{{Path: ".golangci.yml", SHA256: install.Digest([]byte(lint))}})
	if err != nil {
		t.Fatal(err)
	}
	m2Write(t, filepath.Join(dir, ".golangci.yml"), lint)
	m2Write(t, filepath.Join(dir, policyFile), string(data))
}

// m3BirthAnswer simulates the candidate over the birth: the preview names
// every mechanism, an accept writes the strict asset and records it, an
// older go line names the floor and renders nothing, and the scan leaves no
// check unmeasured.
func m3BirthAnswer(t *testing.T, s *suite) m2Answer {
	t.Helper()
	return func(_ context.Context, c command) (m2Reply, bool) {
		if c.program != s.binary || len(c.args) == 0 {
			return m2Reply{}, false
		}
		module, err := os.ReadFile(filepath.Join(c.dir, moduleFile))
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case c.args[0] == commandScan:
			return m2Reply{got: result{code: 1, stdout: `{"schema":1,"checks":[{"id":"policy.makefile","status":"measured"}],"counts":{"unmeasured":0}}`}, target: true}, true
		case slices.Equal(c.args, []string{commandInit}):
			return m2Reply{got: result{stdout: m3PreviewText}, target: true}, true
		case strings.Contains(string(module), "\ngo 1.23\n"):
			return m2Reply{got: result{stdout: "Atualize a diretiva go do go.mod para 1.24 ou mais nova antes de gerar os templates.\n"}, target: true}, true
		case slices.Contains(c.args, "--accept") && !slices.Contains(c.args, "--capabilities"):
			m3WriteAccepted(t, c.dir)
		}
		return m2Reply{target: true}, true
	}
}

func TestM3AdoptionRowsWithASimulatedCandidate(t *testing.T) {
	t.Parallel()
	for _, name := range m3AdoptionNames() {
		s := m2TestSuite(t)
		s.execute = newM2Script(t, s, m3BirthAnswer(t, s)).Execute
		if err := s.adoptionM3(t.Context(), name); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if err := m2TestSuite(t).adoptionM3(t.Context(), "absent"); err == nil {
		t.Fatal("an unknown adoption case must fail")
	}
}
