package policyfiles_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestDependabotCooldown(t *testing.T) {
	t.Parallel()
	const id = "policy.dependabot-cooldown"
	const path = ".github/dependabot.yml"
	const code = "dependabot-cooldown-short"
	red := depsProof(t, id, "red")
	near := depsProof(t, id, "near")
	body := near.files[path]
	assertDeps(t, policyfiles.DependabotCooldown(), []depsCase{
		{name: "six days red", input: red, want: []string{code}},
		{name: "seven and thirty near", input: near},
		{
			name: "stronger floors",
			input: depsEdit(near, path,
				strings.ReplaceAll(strings.ReplaceAll(body, "default-days: 7", "default-days: 14"),
					"semver-major-days: 30", "semver-major-days: 45")),
		},
		{
			name:  "major override red",
			input: depsEdit(near, path, strings.ReplaceAll(body, "semver-major-days: 30", "semver-major-days: 29")),
			want:  []string{code},
		},
		{
			name: "patch override red",
			input: depsEdit(near, path, strings.ReplaceAll(body,
				"semver-major-days: 30", "semver-major-days: 30, semver-patch-days: 1")),
			want: []string{code},
		},
		{
			name: "excluded module red",
			input: depsEdit(near, path, strings.ReplaceAll(body,
				"semver-major-days: 30", "semver-major-days: 30, exclude: [example.test/a]")),
			want: []string{code},
		},
		{
			name: "anchor near",
			input: depsEdit(near, path,
				"version: 2\nx-cooldown: &floor {default-days: 7, semver-major-days: 30}\n"+
					"updates:\n  - package-ecosystem: gomod\n    cooldown: *floor\n"),
		},
		{
			name:   "absent optional configuration",
			input:  fixture{files: map[string]string{}},
			status: model.StatusNotApplicable, reason: "no Dependabot configuration",
		},
		{
			name:   "unsupported version",
			input:  depsEdit(near, path, strings.ReplaceAll(body, "version: 2", "version: 3")),
			status: model.StatusUnmeasured, reason: path + ":1:", outside: true,
		},
		{
			name: "wrong cooldown shape",
			input: depsEdit(near, path,
				"version: 2\nupdates:\n  - package-ecosystem: gomod\n    cooldown: []\n"),
			status: model.StatusUnmeasured, reason: "cooldown", outside: true,
		},
		{
			name: "syntax error", input: depsEdit(near, path, "updates: [\n"),
			status: model.StatusUnmeasured, reason: path,
		},
	})
}

// TestDependabotCooldownReadsEitherFileName holds the check to both names
// Dependabot reads, a stated limit when both exist, and leaves out an entry
// that opens no version update, which only security updates use and which
// the cooldown exempts.
func TestDependabotCooldownReadsEitherFileName(t *testing.T) {
	t.Parallel()
	const code = "dependabot-cooldown-short"
	const short = "version: 2\nupdates:\n  - package-ecosystem: gomod\n    directory: /\n"
	const securityOnly = short + "    open-pull-requests-limit: 0\n"
	assertDeps(t, policyfiles.DependabotCooldown(), []depsCase{
		{name: "red the yaml extension", input: fixture{files: map[string]string{".github/dependabot.yaml": short}}, want: []string{code}},
		{
			name: "limit both names", input: fixture{files: map[string]string{".github/dependabot.yaml": short, ".github/dependabot.yml": short}},
			status: model.StatusUnmeasured, outside: true, reason: ".github/dependabot.yaml:1: both .github/dependabot.yml and .github/dependabot.yaml",
		},
		{name: "near security updates only", input: fixture{files: map[string]string{".github/dependabot.yml": securityOnly}}},
		{name: "red a positive pull request limit", input: fixture{files: map[string]string{".github/dependabot.yml": short + "    open-pull-requests-limit: 5\n"}}, want: []string{code}},
	})
}
