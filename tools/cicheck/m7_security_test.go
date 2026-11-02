package cicheck_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

var errM7Security = errors.New("docs/security.md generators")

func m7SecurityStatements() []string {
	return []string{
		"## Generators in the hook",
		"The agent hooks may write into the checkout only at a post event",
		"only the regenerable files the manifest lists for the changed contract",
		"never golaw.yml, never .golaw/gen.yml",
		"only from the kit's tool directory with a matching receipt",
		"only when the repository's sqlc.yaml and oapi-codegen configuration equal the managed hashes recorded in golaw.yml",
		"No go command, no go generate, no make, no goverter and no golangci-lint run with package loading starts from a hook",
		"regeneration deferred: run golaw gen",
		"a shell write to a generated path that is not literal is fed back, not denied",
	}
}

func m7SecurityFits(text string) error {
	section, found := strings.CutPrefix(text[max(strings.Index(text, "## Generators in the hook"), 0):], "## Generators in the hook")
	if !found {
		return fmt.Errorf("%w: the section is missing", errM7Security)
	}
	section, _, _ = strings.Cut(section, "\n## ")
	for _, statement := range m7SecurityStatements()[1:] {
		if !strings.Contains(section, statement) {
			return fmt.Errorf("%w: the section lacks %q", errM7Security, statement)
		}
	}
	return nil
}

func TestM7SecurityHookContract(t *testing.T) {
	t.Parallel()
	text := string(kitBytes(t, "docs/security.md"))
	rows := []struct {
		name, text string
		bad        bool
	}{
		{"repository", text, false},
		{"red-manifest-writable", strings.Replace(text, "never golaw.yml, never .golaw/gen.yml", "never golaw.yml", 1), true},
		{"red-receipt-dropped", strings.Replace(text, "only from the kit's tool directory with a matching receipt", "from PATH", 1), true},
		{"red-no-section", strings.Replace(text, "## Generators in the hook", "## Generators", 1), true},
		{"near-a-later-section", text + "\n## Later\n\nA later note.\n", false},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if err := m7SecurityFits(row.text); (err != nil) != row.bad {
				t.Fatalf("security error = %v, want error %v", err, row.bad)
			}
		})
	}
}
