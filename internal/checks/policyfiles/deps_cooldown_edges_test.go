package policyfiles_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestDependabotCooldownEffectiveFloors(t *testing.T) {
	t.Parallel()
	const code = "dependabot-cooldown-short"
	assertDeps(t, policyfiles.DependabotCooldown(), []depsCase{
		{name: "missing gomod cooldown", input: cooldownEdgeUpdate("gomod", ""), want: []string{code}},
		{name: "missing Actions cooldown", input: cooldownEdgeUpdate("github-actions", ""), want: []string{code}},
		{name: "major inherits thirty", input: cooldownEdgeUpdate("gomod", "{default-days: 30}")},
		{name: "major inherits seven", input: cooldownEdgeUpdate("gomod", "{default-days: 7}"), want: []string{code}},
		{
			name: "minor override below seven", want: []string{code},
			input: cooldownEdgeUpdate("gomod", "{default-days: 30, semver-minor-days: 6}"),
		},
		{
			name: "Actions has no Go semver floors",
			input: cooldownEdgeUpdate("github-actions",
				"{default-days: 7, semver-major-days: 0, semver-minor-days: 0, semver-patch-days: 0}"),
		},
		{
			name: "irrelevant Actions semver types",
			input: cooldownEdgeUpdate("github-actions",
				"{default-days: 7, semver-major-days: [], semver-minor-days: null, semver-patch-days: 'short'}"),
		},
		{name: "include wildcard", input: cooldownEdgeUpdate("gomod", "{default-days: 30, include: ['*']}")},
		{
			name:  "include wildcard and another entry",
			input: cooldownEdgeUpdate("gomod", "{default-days: 30, include: [example.test/a, '*']}"),
		},
		{
			name: "restrictive include", want: []string{code},
			input: cooldownEdgeUpdate("gomod", "{default-days: 30, include: [example.test/a]}"),
		},
		{
			name:  "empty include and exclude",
			input: cooldownEdgeUpdate("gomod", "{default-days: 30, include: [], exclude: []}"),
		},
		{
			name: "exclude wildcard", want: []string{code},
			input: cooldownEdgeUpdate("gomod", "{default-days: 30, exclude: ['*']}"),
		},
		{name: "negative age is a finding", input: cooldownEdgeUpdate("gomod", "{default-days: -1}"), want: []string{code}},
		{name: "other ecosystem cooldown ignored", input: cooldownEdgeUpdate("npm", "[unrecognized, shape]")},
		{name: "empty updates", input: cooldownEdgeDocument("version: 2\nupdates: []\n")},
		{
			name: "other ecosystem duplicate cooldown ignored",
			input: cooldownEdgeDocument("version: 2\nupdates:\n  - package-ecosystem: npm\n" +
				"    cooldown: {}\n    cooldown: []\n"),
		},
		{
			name:  "Actions duplicate semver keys ignored",
			input: cooldownEdgeUpdate("github-actions", "{default-days: 7, semver-major-days: 1, semver-major-days: 2}"),
		},
		{
			name: "unrelated duplicate metadata ignored",
			input: cooldownEdgeDocument("version: 2\nx-note: first\nx-note: second\nupdates:\n" +
				"  - package-ecosystem: gomod\n    directory: /first\n    directory: /second\n" +
				"    cooldown: {default-days: 30, x-note: first, x-note: second}\n"),
		},
	})
}

func TestDependabotCooldownAcceptsYAMLAliasesAndIntegers(t *testing.T) {
	t.Parallel()
	assertDeps(t, policyfiles.DependabotCooldown(), []depsCase{
		{
			name: "integer alias and hexadecimal",
			input: cooldownEdgeDocument("version: 2\nx-days: &days 0x1e\nupdates:\n" +
				"  - package-ecosystem: gomod\n    cooldown: {default-days: *days}\n"),
		},
		{
			name: "updates sequence alias",
			input: cooldownEdgeDocument("version: 2\nx-updates: &updates\n" +
				"  - package-ecosystem: gomod\n    cooldown: {default-days: 30}\nupdates: *updates\n"),
		},
		{
			name: "include sequence alias",
			input: cooldownEdgeDocument("version: 2\nx-all: &all ['*']\nupdates:\n" +
				"  - package-ecosystem: gomod\n    cooldown: {default-days: 30, include: *all}\n"),
		},
		{
			name: "merged cooldown own override wins", want: []string{"dependabot-cooldown-short"},
			input: cooldownEdgeDocument("version: 2\nx-floor: &floor {default-days: 7, semver-major-days: 30}\n" +
				"updates:\n  - package-ecosystem: gomod\n" +
				"    cooldown: {<<: *floor, semver-major-days: 29}\n"),
		},
	})
}

func TestDependabotCooldownUnsupportedValuesStayUnmeasured(t *testing.T) {
	t.Parallel()
	const header = "version: 2\nupdates:\n  - package-ecosystem: gomod\n    cooldown: "
	rows := []struct {
		name string
		body string
		line int
	}{
		{name: "version string", body: "version: '2'\nupdates: []\n", line: 1},
		{name: "version null", body: "version: null\nupdates: []\n", line: 1},
		{name: "updates mapping", body: "version: 2\nupdates: {}\n", line: 2},
		{name: "updates null", body: "version: 2\nupdates: null\n", line: 2},
		{name: "update scalar", body: "version: 2\nupdates:\n  - gomod\n", line: 3},
		{name: "ecosystem number", body: "version: 2\nupdates:\n  - package-ecosystem: 1\n", line: 3},
		{name: "ecosystem null", body: "version: 2\nupdates:\n  - package-ecosystem: null\n", line: 3},
		{name: "cooldown scalar", body: header + "seven\n", line: 4},
		{name: "cooldown null", body: header + "null\n", line: 4},
		{name: "numeric string", body: header + "{default-days: '30'}\n", line: 4},
		{name: "integer overflow", body: header + "{default-days: 9223372036854775808}\n", line: 4},
		{name: "floating days", body: header + "{default-days: 30.0}\n", line: 4},
		{name: "null days", body: header + "{default-days: null}\n", line: 4},
		{name: "mapping days", body: header + "{default-days: {value: 30}}\n", line: 4},
		{name: "include scalar", body: header + "{default-days: 30, include: '*'}\n", line: 4},
		{name: "include numeric entry", body: header + "{default-days: 30, include: [123]}\n", line: 4},
		{name: "include null", body: header + "{default-days: 30, include: null}\n", line: 4},
		{name: "exclude nested sequence", body: header + "{default-days: 30, exclude: [[]]}\n", line: 4},
		{name: "missing version", body: "updates: []\n", line: 1},
		{name: "missing updates", body: "version: 2\n", line: 1},
		{name: "missing ecosystem", body: "version: 2\nupdates:\n  - directory: /\n", line: 3},
		{
			name: "ecosystem wrong type names its own line",
			body: "version: 2\nupdates:\n  - directory: /\n    package-ecosystem: []\n", line: 4,
		},
		{name: "duplicate version", body: "version: 2\nversion: 2\nupdates: []\n", line: 2},
		{name: "duplicate updates", body: "version: 2\nupdates: []\nupdates: []\n", line: 3},
		{
			name: "duplicate ecosystem", line: 4,
			body: "version: 2\nupdates:\n  - package-ecosystem: gomod\n" +
				"    package-ecosystem: github-actions\n    cooldown: {default-days: 7}\n",
		},
		{
			name: "duplicate cooldown", line: 5,
			body: header + "{default-days: 7}\n    cooldown: {default-days: 30}\n",
		},
		{
			name: "duplicate default days", line: 6,
			body: header + "\n      default-days: 7\n      default-days: 30\n",
		},
		{
			name: "duplicate include", line: 7,
			body: header + "\n      default-days: 30\n      include: ['*']\n      include: [example.test/a]\n",
		},
		{
			name: "later unsupported entry discards earlier findings",
			body: header + "{default-days: 1}\n  - package-ecosystem: gomod\n" +
				"    cooldown: {default-days: '30'}\n", line: 6,
		},
	}
	cases := make([]depsCase, 0, len(rows))
	for _, row := range rows {
		cases = append(cases, depsCase{
			name: row.name, input: cooldownEdgeDocument(row.body), status: model.StatusUnmeasured,
			reason: fmt.Sprintf(".github/dependabot.yml:%d:", row.line), outside: true,
		})
	}
	assertDeps(t, policyfiles.DependabotCooldown(), cases)
}

func TestDependabotCooldownNamesEachDefectiveUpdateOnce(t *testing.T) {
	t.Parallel()
	input := cooldownEdgeDocument("version: 2\nupdates:\n  - package-ecosystem: gomod\n" +
		"    cooldown: {default-days: 1, semver-major-days: 2, semver-minor-days: 3, " +
		"semver-patch-days: 4, include: [example.test/a], exclude: [example.test/b]}\n" +
		"  - package-ecosystem: github-actions\n    cooldown: {default-days: 6}\n" +
		"  - package-ecosystem: npm\n    cooldown: [unsupported, but, irrelevant]\n")
	got := measure(t, policyfiles.DependabotCooldown(), input)
	if got.Status.Kind != model.StatusMeasured || len(got.Findings) != 2 {
		t.Fatalf("result = %+v, want one finding per defective supported update", got)
	}
	for index, want := range []struct {
		line int
		keys []string
	}{
		{line: 3, keys: []string{"gomod", "default-days", "semver-major-days", "semver-minor-days", "semver-patch-days", "include", "exclude"}},
		{line: 5, keys: []string{"github-actions", "default-days"}},
	} {
		found := got.Findings[index]
		if found.Path != ".github/dependabot.yml" || found.Line != want.line || found.Code != "dependabot-cooldown-short" {
			t.Errorf("finding = %+v, want configuration location at line %d", found, want.line)
		}
		for _, key := range want.keys {
			if !strings.Contains(found.Message, key) {
				t.Errorf("message = %q, want actionable key %q", found.Message, key)
			}
		}
	}
}

func TestDependabotCooldownAbsenceAndReadBound(t *testing.T) {
	t.Parallel()
	got := measure(t, policyfiles.DependabotCooldown(), fixture{files: map[string]string{}})
	const reason = "no Dependabot configuration at .github/dependabot.yml; " +
		"init writes it unless CODEOWNERS gives .github/ to another team"
	if got.Status.Kind != model.StatusNotApplicable || got.Status.Reason != reason || len(got.Findings) != 0 {
		t.Fatalf("result = %+v, want exact optional configuration reason %q", got, reason)
	}
	assertDeps(t, policyfiles.DependabotCooldown(), []depsCase{{
		name: "read bound", input: cooldownEdgeDocument(strings.Repeat("x", model.MaxReadBytes+1)),
		status: model.StatusUnmeasured, reason: model.ErrTooLarge.Error(),
	}})
}

func cooldownEdgeUpdate(ecosystem, cooldown string) fixture {
	body := "version: 2\nupdates:\n  - package-ecosystem: " + ecosystem + "\n"
	if cooldown != "" {
		body += "    cooldown: " + cooldown + "\n"
	}
	return cooldownEdgeDocument(body)
}

func cooldownEdgeDocument(body string) fixture {
	return fixture{files: map[string]string{".github/dependabot.yml": body}}
}

func TestDependabotCooldownNamesOnlyNecessaryRepairs(t *testing.T) {
	t.Parallel()
	const gomod = "set the cooldown of the gomod update in .github/dependabot.yml to "
	const actions = "set the cooldown of the github-actions update in .github/dependabot.yml to "
	for _, row := range []struct {
		name, ecosystem, cooldown, want string
	}{
		{
			name: "missing cooldown inherits minor and patch", ecosystem: "gomod",
			want: gomod + "default-days 7 or more and semver-major-days 30 or more",
		},
		{
			name: "short default fixes omitted minor and patch", ecosystem: "gomod",
			cooldown: "{default-days: 6, semver-major-days: 30}", want: gomod + "default-days 7 or more",
		},
		{
			name: "only the written minor is short", ecosystem: "gomod",
			cooldown: "{default-days: 30, semver-minor-days: 6}", want: gomod + "semver-minor-days 7 or more",
		},
		{
			name: "written minor and patch need separate repairs", ecosystem: "gomod",
			cooldown: "{default-days: 6, semver-minor-days: 1, semver-patch-days: 2, semver-major-days: 30}",
			want:     gomod + "default-days 7 or more, semver-minor-days 7 or more and semver-patch-days 7 or more",
		},
		{
			name: "major inherits a short default", ecosystem: "gomod", cooldown: "{default-days: 7}",
			want: gomod + "semver-major-days 30 or more",
		},
		{
			name: "short Actions default", ecosystem: "github-actions", cooldown: "{default-days: 6}",
			want: actions + "default-days 7 or more",
		},
		{
			name: "remove exclude with the floors", ecosystem: "gomod",
			cooldown: "{default-days: 6, exclude: [example.test/a]}",
			want:     gomod + "default-days 7 or more, semver-major-days 30 or more and no exclude list",
		},
		{
			name: "broaden or remove include", ecosystem: "gomod",
			cooldown: "{default-days: 30, include: [example.test/a]}", want: gomod + "include ['*'] or no include list",
		},
		{
			name: "repair both filters", ecosystem: "github-actions",
			cooldown: "{default-days: 7, exclude: [example.test/a], include: [example.test/b]}",
			want:     actions + "no exclude list and include ['*'] or no include list",
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got := measure(t, policyfiles.DependabotCooldown(), cooldownEdgeUpdate(row.ecosystem, row.cooldown))
			if got.Status.Kind != model.StatusMeasured || len(got.Findings) != 1 {
				t.Fatalf("result = %+v, want exactly one measured finding", got)
			}
			if got.Findings[0].Message != row.want {
				t.Fatalf("message = %q, want %q", got.Findings[0].Message, row.want)
			}
		})
	}
}
