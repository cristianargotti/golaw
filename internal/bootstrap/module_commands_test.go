package bootstrap_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/install"
)

// TestTemplateDiffNamesTheGoGetOfAGoModProposal holds the go.mod proposal
// to a repair the person runs as printed: go get writes the require and the
// go.sum lines the build needs, which the printed require alone does not.
func TestTemplateDiffNamesTheGoGetOfAGoModProposal(t *testing.T) {
	t.Parallel()
	const module = "module example.test/service\n\ngo 1.27\n"
	const command = "go get github.com/quasilyte/go-ruleguard/dsl@v0.3.23"
	const require = module + "require github.com/quasilyte/go-ruleguard/dsl v0.3.23\n"
	for _, tc := range []struct {
		name, language, path, before, want string
	}{
		{"red go.mod require", "pt-BR", "go.mod", module, "rode " + command + " e depois go mod tidy, que gravam também o go.sum"},
		{"red english", "en", "go.mod", module, "run " + command + " and then go mod tidy, which also write go.sum"},
		{"near require already there", "pt-BR", "go.mod", require, ""},
		{"near other proposal", "pt-BR", "Makefile", module, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			plan := bootstrap.TemplatePlan{Language: tc.language, Proposed: []install.Change{
				{Path: tc.path, Before: []byte(tc.before), After: []byte(require), Existed: true},
			}}
			var out bytes.Buffer
			if err := bootstrap.WriteTemplateDiff(&out, plan); err != nil {
				t.Fatal(err)
			}
			if got := strings.Contains(out.String(), "go get "); got != (tc.want != "") || !strings.Contains(out.String(), tc.want) {
				t.Fatalf("output = %q, want %q", out.String(), tc.want)
			}
		})
	}
}
