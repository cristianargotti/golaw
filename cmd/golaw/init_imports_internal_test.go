package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
)

func TestInitRecordsTheDirectSetTheImportsUse(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	repo.Commit("chore: establish application", map[string]string{
		"go.mod": "module example.test/birth\n\ngo 1.27\n\nrequire (\n\texample.test/web v1.12.0\n)\n\n" +
			"require (\n\texample.test/cli v1.10.2 // indirect\n\texample.test/yaml v1.0.0 // indirect\n)\n",
		"main.go": "package main\n\nimport _ \"example.test/cli\"\n\nfunc main() {}\n",
	})
	ta := initBindApp(t)
	if code := ta.run(t.Context(), []string{"init", repo.Root}); code != exitClean {
		t.Fatalf("dry run exit = %d: %s", code, ta.stderr.String())
	}
	out := ta.stdout.String()
	for _, want := range []string{
		"+  - example.test/cli\n", "+  - example.test/web\n",
		"go.mod marca como indiretos módulos que o código importa diretamente: example.test/cli.",
		"rode go mod tidy",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("dry run lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "+  - example.test/yaml\n") {
		t.Fatalf("an indirect module nothing imports entered the baseline:\n%s", out)
	}
}

func TestInitKeepsAnExistingBaselineWithoutAnImportedNotice(t *testing.T) {
	t.Parallel()
	for _, decisions := range []string{"docs/adr", "docs/decisions"} {
		t.Run(decisions, func(t *testing.T) {
			t.Parallel()
			repo := gittest.New(t)
			repo.Commit("chore: establish application", map[string]string{
				"go.mod": "module example.test/birth\n\ngo 1.27\n\nrequire (\n\texample.test/web v1.12.0\n)\n\n" +
					"require (\n\texample.test/cli v1.10.2 // indirect\n\texample.test/yaml v1.0.0 // indirect\n)\n",
				"main.go": "package main\n\nimport _ \"example.test/cli\"\n\nfunc main() {}\n",
				decisions + "/0001-inherited-baseline.md": "# Existing baseline\n",
			})
			ta := initBindApp(t)
			if code := ta.run(t.Context(), []string{"init", repo.Root}); code != exitClean {
				t.Fatalf("dry run exit = %d: %s", code, ta.stderr.String())
			}
			if strings.Contains(ta.stdout.String(), "importa diretamente") {
				t.Fatalf("kept baseline received an imported notice:\n%s", ta.stdout.String())
			}
		})
	}
}

func TestInitAppliesImportedBaselineWithoutEditingTheModuleOrArmingRungs(t *testing.T) {
	t.Parallel()
	for _, language := range []string{"pt-BR", "en"} {
		t.Run(language, func(t *testing.T) {
			t.Parallel()
			repo := gittest.New(t)
			module := "module example.test/birth\n\ngo 1.27\n\nrequire example.test/cli v1.10.2 // indirect\n"
			repo.Commit("chore: establish application", map[string]string{
				"go.mod": module, "main.go": "package main\n\nimport _ \"example.test/cli\"\n",
			})
			ta := initBindApp(t)
			if code := ta.run(t.Context(), []string{"init", "--apply", "--language", language, repo.Root}); code != exitClean {
				t.Fatalf("apply exit = %d: %s", code, ta.stderr.String())
			}
			requireImportedBaseline(t, repo.Root, module)
			remedy := "rode go mod tidy"
			if language == "en" {
				remedy = "run go mod tidy"
			}
			if !strings.Contains(ta.stdout.String(), remedy) {
				t.Fatalf("apply lacks %q: %s", remedy, ta.stdout.String())
			}
			requireUnarmed(t, repo.GitDir())
		})
	}
}

func requireImportedBaseline(t *testing.T, root, module string) {
	t.Helper()
	baseline, err := os.ReadFile(filepath.Join(root, "docs", "adr", "0001-inherited-baseline.md"))
	if err != nil || !strings.Contains(string(baseline), "  - example.test/cli\n") ||
		!strings.Contains(string(baseline), "| example.test/cli | v1.10.2 | inherited |") {
		t.Fatalf("baseline = %q, error %v", baseline, err)
	}
	after, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil || string(after) != module {
		t.Fatalf("module changed = %q, error %v", after, err)
	}
}
