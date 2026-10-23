package cicheck_test

import (
	"strings"
	"testing"
)

// TestM3AdoptionStagesWhatInitRenders holds the adoption sequence of the
// setup guide and the README to the core files an M3 init renders and to
// the go.mod repair the rules file needs, so a person who stages the list
// as printed leaves no rendered file untracked and lint loads its rules.
func TestM3AdoptionStagesWhatInitRenders(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"docs/setup.md", "README.md"} {
		text := string(kitBytes(t, path))
		apply := strings.Index(text, "golaw init --apply\n")
		get := strings.Index(text, "go get github.com/quasilyte/go-ruleguard/dsl@v0.3.23\ngo mod tidy\n")
		stage := strings.Index(text, "git add golaw.yml")
		if apply < 0 || get < apply || stage < get {
			t.Errorf("%s: the go get and go mod tidy of the rules module must run between init --apply and git add", path)
			continue
		}
		line := strings.ReplaceAll(text[stage:stage+strings.Index(text[stage:], "\ngit commit")], "\\\n", " ")
		for _, file := range []string{" .github/workflows/ci.yml ", " scripts/lint/rules.go ", " go.mod ", " go.sum ", " tools/go.mod "} {
			if !strings.Contains(line+" ", file) {
				t.Errorf("%s: the adoption staging line lacks%s", path, file)
			}
		}
	}
}
