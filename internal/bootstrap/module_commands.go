package bootstrap

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/install"
)

// writeModuleCommands names, under a go.mod proposal, the go get of the
// requires it adds and the go mod tidy after it: go.mod stays the person's
// file, go get writes the go.sum lines the build needs, which the printed
// require alone does not, and tidy drops the indirect mark go get leaves on
// a module only a build-tagged file imports.
func writeModuleCommands(out io.Writer, plan TemplatePlan) error {
	index := slices.IndexFunc(plan.Proposed, func(c install.Change) bool { return c.Path == goModule })
	if index < 0 {
		return nil
	}
	change := plan.Proposed[index]
	before := strings.Split(string(change.Before), "\n")
	var modules []string
	for line := range strings.SplitSeq(string(change.After), "\n") {
		if fields := strings.Fields(line); len(fields) == 3 && fields[0] == "require" && !slices.Contains(before, line) {
			modules = append(modules, fields[1]+"@"+fields[2])
		}
	}
	if len(modules) == 0 {
		return nil
	}
	text := "Para aplicar a proposta de go.mod, rode go get %s e depois go mod tidy, que gravam também o go.sum, e inclua go.mod e go.sum no commit\n"
	if english(plan.Language) {
		text = "To apply the go.mod proposal, run go get %s and then go mod tidy, which also write go.sum, and commit go.mod and go.sum\n"
	}
	if _, err := fmt.Fprintf(out, text, strings.Join(modules, " ")); err != nil {
		return fmt.Errorf("bootstrap: write the go.mod command: %w", err)
	}
	return nil
}
