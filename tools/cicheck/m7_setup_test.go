package cicheck_test

import (
	"strings"
	"testing"
)

func m7SetupStatements() []string {
	return []string{
		"## Generators",
		"golaw setup --gen builds sqlc, oapi-codegen and golangci-lint from the tools module of the repository",
		"A repeated golaw setup --gen builds nothing",
		"A missing generator is a doctor line and a notice, never a denial",
		"init never arms it",
		"ladder: { hook_regenerate: false }",
		"docs/generate.md",
	}
}

const m7ReadmeLine = "- Generators, golaw new, golaw gen and golaw map: `docs/generate.md`."

// m7SetupFits holds the setup section and the README pointer; the section
// names no Sonar tool, which stays out of every local step (decision 9).
func m7SetupFits(setup, readme string) bool {
	for _, statement := range m7SetupStatements() {
		if !strings.Contains(setup, statement) {
			return false
		}
	}
	at := strings.Index(setup, "## Generators")
	section, _, _ := strings.Cut(setup[at:], "\n## ")
	return strings.Contains(readme, m7ReadmeLine) && !strings.Contains(section, "sonar-scanner") && !strings.Contains(section, "SONAR_TOKEN")
}

func TestM7SetupGenerators(t *testing.T) {
	t.Parallel()
	setup, readme := string(kitBytes(t, "docs/setup.md")), string(kitBytes(t, "README.md"))
	rows := []struct {
		name, setup, readme string
		want                bool
	}{
		{"repository", setup, readme, true},
		{"red-armed-by-init", strings.Replace(setup, "init never arms it", "init arms it", 1), readme, false},
		{"red-a-denial", strings.Replace(setup, "a notice, never a denial: the hook leaves", "a notice, a denial: the hook leaves", 1), readme, false},
		{"red-sonar-named", strings.Replace(setup, "## Generators\n", "## Generators\n\nRun sonar-scanner after it.\n", 1), readme, false},
		{"red-no-pointer", setup, strings.Replace(readme, m7ReadmeLine, "", 1), false},
		{"near-later-section", setup + "\n## Later\n\nsonar-scanner is a bench instrument.\n", readme, true},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if got := m7SetupFits(row.setup, row.readme); got != row.want {
				t.Fatalf("setup fits = %v, want %v", got, row.want)
			}
		})
	}
}
