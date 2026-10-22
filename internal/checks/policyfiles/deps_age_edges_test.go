package policyfiles_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

// depsAgeChain is a deps-age target that delegates through levels targets
// before the last one runs the recipe.
func depsAgeChain(levels int, recipe string) string {
	lines := []string{"deps-age: l1"}
	for level := 1; level < levels; level++ {
		lines = append(lines, "l"+strconv.Itoa(level)+": l"+strconv.Itoa(level+1))
	}
	lines = append(lines, "l"+strconv.Itoa(levels)+":", "\t"+recipe, "")
	return strings.Join(lines, "\n")
}

// TestDepsAgeConfigWalksDelegationWithinItsBounds pins the delegation walk:
// the recipe of the target and of its literal prerequisites in make's
// order, each target once and eight levels at most, the first depsage
// invocation judged, a limit kept only when nothing proves the floor.
func TestDepsAgeConfigWalksDelegationWithinItsBounds(t *testing.T) {
	t.Parallel()
	const id = "policy.deps-age-config"
	const code = "deps-age-minimum"
	const seven = "go run ./tools/depsage -days 7"
	red := depsProof(t, id, "red", "go")
	makefile := func(content string) fixture { return depsEdit(red, "Makefile", content) }
	assertDeps(t, policyfiles.DepsAgeConfig(), []depsCase{
		{name: "recipe before an unknown prerequisite", input: makefile("deps-age: lint\n\t" + seven + "\nlint:\n\tgolangci-lint run ./...\n")},
		{
			name:   "unknown prerequisite without an invocation",
			input:  makefile("deps-age: lint\n\techo done\nlint:\n\tgolangci-lint run ./...\n"),
			status: model.StatusUnmeasured, reason: "Makefile:3: golangci-lint", outside: true,
		},
		{name: "eight levels", input: makefile(depsAgeChain(8, seven))},
		{
			name: "nine levels", input: makefile(depsAgeChain(9, seven)),
			status: model.StatusUnmeasured, reason: "deeper than eight levels", outside: true,
		},
		{name: "cycle without a recipe", input: makefile("deps-age: a\na: b\nb: a\n"), want: []string{code}},
		{name: "file prerequisite", input: makefile("deps-age: go.sum\n\t" + seven + "\n")},
		{
			name: "pattern could supply the prerequisite", input: makefile("deps-age: age.out\n%.out:\n\t" + seven + "\n"),
			status: model.StatusUnmeasured, reason: "%.out", outside: true,
		},
		{name: "no recipe and no prerequisite", input: makefile("deps-age:\n"), want: []string{code}},
		{name: "first invocation wins", input: makefile("deps-age:\n\tgo run ./tools/depsage -days 3\n\tgo run ./tools/depsage -days 9\n"), want: []string{code}},
		{name: "setup line before the check", input: makefile("deps-age:\n\tgo mod download\n\t" + seven + "\n")},
		{
			name:  "target-specific export as golaw.mk writes it",
			input: makefile("deps-age: export GOTOOLCHAIN := auto\ndeps-age:\n\tgo run github.com/cristianargotti/golaw/tools/depsage@v0.2.0 -days 7\n"),
		},
		{name: "ignored failure in a delegated target", input: makefile("deps-age: age-check\nage-check:\n\t-" + seven + "\n"), want: []string{code}},
		{name: "missing include beside a present target", input: makefile("include golaw.mk\ndeps-age:\n\t" + seven + "\n")},
		{
			name:   "missing include under a delegated prerequisite",
			input:  makefile("include golaw.mk\ndeps-age: age-check\n"),
			status: model.StatusUnmeasured, reason: "golaw.mk",
		},
		{
			name:   "read bound",
			input:  makefile(strings.Repeat("x", model.MaxReadBytes+1)),
			status: model.StatusUnmeasured, reason: model.ErrTooLarge.Error(),
		},
		{name: "staged scan judges the index", input: fixture{
			capabilities: []string{"go"}, mode: model.ModeStaged, files: map[string]string{"Makefile": red.files["Makefile"]},
		}, want: []string{code}},
	})
}

// TestDepsAgeConfigNamesWhatToFix pins the file, the line and the message
// of each finding, so a person or an agent fixes the target without help.
func TestDepsAgeConfigNamesWhatToFix(t *testing.T) {
	t.Parallel()
	red := depsProof(t, "policy.deps-age-config", "red", "go")
	for _, tc := range []struct {
		name       string
		files      map[string]string
		path       string
		line       int
		message    string
		wantStatus model.StatusKind
	}{
		{
			name:  "below seven in an included file",
			files: map[string]string{"Makefile": "include mk/age.mk\ndeps-age: age-check\n", "mk/age.mk": "age-check:\n\tgo run ./tools/depsage -days 3\n"},
			path:  "mk/age.mk", line: 1, message: "effective minimum age of 3 days; pass -days 7 or more and let its failure stop the target",
		},
		{
			name:  "ignored failure",
			files: map[string]string{"Makefile": "deps-age:\n\t-go run ./tools/depsage -days 7\n"},
			path:  "Makefile", line: 1, message: "effective minimum age of 0 days",
		},
		{
			name:  "no invocation",
			files: map[string]string{"Makefile": "\ndeps-age:\n\techo checked\n"},
			path:  "Makefile", line: 2, message: "runs no depsage check; run the kit's depsage with -days 7 from it, as golaw.mk does",
		},
		{
			name:  "absent target",
			files: map[string]string{"Makefile": "build:\n\tgo build ./...\n"},
			path:  "Makefile", line: 0, message: "add the deps-age target to Makefile or an included file",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := measure(t, policyfiles.DepsAgeConfig(), fixture{capabilities: []string{"go"}, files: tc.files})
			if got.Status.Kind != model.StatusMeasured || len(got.Findings) != 1 || !strings.HasPrefix(got.Scope, "deps-age target of Makefile") {
				t.Fatalf("result = %+v", got)
			}
			f := got.Findings[0]
			if f.Path != tc.path || f.Line != tc.line || !strings.Contains(f.Message, tc.message) || f.Code != "deps-age-minimum" {
				t.Fatalf("finding = %+v, want %s:%d %q", f, tc.path, tc.line, tc.message)
			}
		})
	}
	if got := measure(t, policyfiles.DepsAgeConfig(), red); len(got.Findings) != 1 || got.Findings[0].Line != 1 || !strings.Contains(got.Findings[0].Message, "6 days") {
		t.Fatalf("red fixture = %+v", got)
	}
}
