package policyfiles_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestDepsAgeConfig(t *testing.T) {
	t.Parallel()
	const id = "policy.deps-age-config"
	const code = "deps-age-minimum"
	red := depsProof(t, id, "red", "go")
	near := depsProof(t, id, "near", "go")
	for _, tc := range []struct {
		name, recipe string
		want         []string
		status       model.StatusKind
		outside      bool
	}{
		{name: "default seven", recipe: "go run ./tools/depsage"},
		{name: "stronger", recipe: "go run ./tools/depsage -days=14"},
		{name: "quiet prefix", recipe: "@go run ./tools/depsage -days 7"},
		{name: "toolchain switch near", recipe: "GOTOOLCHAIN=auto go run github.com/cristianargotti/golaw/tools/depsage@v0.3.0 -days 7"},
		{
			name: "computed environment limit", recipe: "GOTOOLCHAIN=$(TOOLCHAIN) go run ./tools/depsage -days 7",
			status: model.StatusUnmeasured, outside: true,
		},
		{name: "last option wins red", recipe: "go run ./tools/depsage -days 7 -days 1", want: []string{code}},
		{name: "last option wins near", recipe: "go run ./tools/depsage -days 1 -days 7"},
		{name: "echo is not execution", recipe: "echo go run ./tools/depsage -days 7", want: []string{code}},
		{name: "ignored exit red", recipe: "-go run ./tools/depsage -days 7", want: []string{code}},
		{
			name: "shell expression limit", recipe: "go run ./tools/depsage -days $(shell age)",
			status: model.StatusUnmeasured, outside: true,
		},
		{
			name: "wrapper limit", recipe: "sh -c 'go run ./tools/depsage -days 7'",
			status: model.StatusUnmeasured, outside: true,
		},
	} {
		assertDeps(t, policyfiles.DepsAgeConfig(), []depsCase{{
			name: tc.name, input: depsEdit(red, "Makefile", "deps-age:\n\t"+tc.recipe+"\n"),
			want: tc.want, status: tc.status, outside: tc.outside,
		}})
	}
	assertDeps(t, policyfiles.DepsAgeConfig(), []depsCase{
		{name: "six red", input: red, want: []string{code}},
		{name: "included target near", input: near},
		{
			name: "missing target", input: depsEdit(red, "Makefile", "build:\n\tgo build ./...\n"),
			want: []string{code},
		},
		{name: "absent Makefile near, named by policy.makefile", input: fixture{capabilities: []string{"go"}, files: map[string]string{}}},
		{
			name:  "delegated through a prerequisite near",
			input: depsEdit(red, "Makefile", "deps-age: age-check\nage-check:\n\tgo run ./tools/depsage -days 7\n"),
		},
		{
			name:  "delegated below seven red",
			input: depsEdit(red, "Makefile", "deps-age: age-check\nage-check:\n\tgo run ./tools/depsage -days 3\n"),
			want:  []string{code},
		},
		{
			name: "delegated to a raw prerequisite limit", input: depsEdit(red, "Makefile", "deps-age: $(AGE_CHECK)\n"),
			status: model.StatusUnmeasured, outside: true,
		},
		{
			name:   "target only inside a conditional block limit",
			input:  depsEdit(red, "Makefile", "ifeq ($(CI),true)\ndeps-age:\n\tgo run ./tools/depsage -days 7\nendif\n"),
			status: model.StatusUnmeasured, outside: true,
		},
		{
			name: "missing include", input: depsEdit(near, "golaw.mk", ""),
			status: model.StatusUnmeasured, reason: "golaw.mk",
		},
		{
			name: "capability off", input: depsProof(t, id, "red"),
			status: model.StatusNotApplicable, reason: "capability go",
		},
	})
}
