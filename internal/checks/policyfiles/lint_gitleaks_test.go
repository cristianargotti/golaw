package policyfiles_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

// gitleaksBase is the gitleaks configuration of the template: the default
// rules and no allowlist.
const gitleaksBase = "title = \"golaw\"\n\n[extend]\nuseDefault = true\n"

func TestGitleaksGenericRefusesAnAllowlistOfGeneratedFiles(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, config string
		want         []string
	}{
		{"the template", gitleaksBase, nil},
		{"near: an allowlist of regexes alone", gitleaksBase + "\n[[allowlists]]\ndescription = \"public node ids\"\nregexes = ['''\\bPVT_[A-Za-z0-9_-]{8,}\\b''']\n", nil},
		{"near: an allowlist of fixtures and lock files", gitleaksBase + "\n[[allowlists]]\npaths = ['''(^|/)testdata/''', '''(^|/)go\\.sum$''']\n", nil},
		{"the generated tree", gitleaksBase + "\n[[allowlists]]\npaths = ['''^gen/''']\n", []string{"gitleaks-generated-allowlist"}},
		{"the legacy global allowlist", gitleaksBase + "\n[allowlist]\npaths = ['''sqlcgen/''']\n", []string{"gitleaks-generated-allowlist"}},
		{"a rule allowlist of generated code", gitleaksBase + "\n[[rules]]\nid = \"token\"\nregex = '''tok_[a-z0-9]{16}'''\n[[rules.allowlists]]\npaths = ['''\\.pb\\.go$''']\n", []string{"gitleaks-generated-allowlist"}},
		{"every Go file", gitleaksBase + "\n[[allowlists]]\npaths = ['''\\.go$''', '''^docs/''']\n", []string{"gitleaks-generated-allowlist"}},
		{"near: a path that does not compile", gitleaksBase + "\n[[allowlists]]\npaths = ['''(gen''']\n", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := measure(t, policyfiles.GitleaksGeneric(), fixture{files: map[string]string{".gitleaks.toml": tc.config}})
			if got := codes(res); res.Status.Kind != model.StatusMeasured || (!slices.Equal(got, tc.want) && len(got)+len(tc.want) != 0) {
				t.Fatalf("status %+v, codes = %v, want %v (%+v)", res.Status, got, tc.want, res.Findings)
			}
		})
	}
}

func TestGitleaksGenericLeavesTheRungsToTheDeveloper(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		files map[string]string
	}{
		{"near: no lefthook.yml, which is never required", map[string]string{".gitleaks.toml": gitleaksBase}},
		{"near: neither file", map[string]string{}},
		{"near: a pre-commit without gitleaks", map[string]string{"lefthook.yml": "pre-commit:\n  commands:\n    fmt:\n      run: gofmt -l .\n"}},
		{"near: gitleaks only before a push", map[string]string{"lefthook.yml": "pre-push:\n  commands:\n    secrets:\n      run: gitleaks git --redact\n"}},
		{"near: the commit-msg rung alone", map[string]string{"lefthook.yml": "commit-msg:\n  commands:\n    golaw:\n      run: golaw hook commit-msg '{1}'\n"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := measure(t, policyfiles.GitleaksGeneric(), fixture{files: tc.files})
			if res.Status.Kind != model.StatusMeasured || len(res.Findings) != 0 {
				t.Fatalf("a local rung is the developer's to arm or trim: %+v", res)
			}
		})
	}
}

func TestGitleaksGenericNamesWhereItCannotJudge(t *testing.T) {
	t.Parallel()
	extend := measure(t, policyfiles.GitleaksGeneric(), fixture{files: map[string]string{".gitleaks.toml": "[extend]\npath = \"base.toml\"\n"}})
	if !model.IsOutsideSubset(extend.Status) || !strings.HasPrefix(extend.Status.Reason, ".gitleaks.toml:1: an [extend] path or url, whose allowlists golaw does not read: ") {
		t.Errorf("an extended configuration = %+v", extend.Status)
	}
	broken := measure(t, policyfiles.GitleaksGeneric(), fixture{files: map[string]string{".gitleaks.toml": "title = \n"}})
	if broken.Status.Kind != model.StatusUnmeasured || model.IsOutsideSubset(broken.Status) || !strings.HasPrefix(broken.Status.Reason, "parse .gitleaks.toml: ") {
		t.Errorf("a configuration that does not parse = %+v", broken.Status)
	}
	res := measure(t, policyfiles.GitleaksGeneric(), fixture{files: map[string]string{".gitleaks.toml": gitleaksBase + "\n[[allowlists]]\npaths = ['''^gen/''']\n"}})
	if len(res.Findings) != 1 || res.Findings[0].Path != ".gitleaks.toml" || res.Findings[0].Message !=
		`allowlist path "^gen/" of .gitleaks.toml matches the generated file gen/client.gen.go, so a secret written there goes unscanned; remove it, because generated code is scanned like any other` {
		t.Fatalf("findings = %+v", res.Findings)
	}
}

func TestGitleaksGenericStatesConditionalAndScopedLimits(t *testing.T) {
	t.Parallel()
	const candidate = "paths = ['^gen/']\n"
	const rule = "[[rules]]\nid = 'token'\nregex = 'tok_[a-z0-9]{16}'\n"
	for _, tc := range []struct{ name, config, construct string }{
		{"AND regexes", "[[allowlists]]\ncondition = 'AND'\nregexes = ['public']\n" + candidate, "AND"},
		{"AND commits", "[[allowlists]]\ncondition = 'AND'\ncommits = ['abc']\n" + candidate, "AND"},
		{"AND stopwords", "[[allowlists]]\ncondition = 'AND'\nstopwords = ['public']\n" + candidate, "AND"},
		{"unknown condition", "[[allowlists]]\ncondition = 'XOR'\n" + candidate, "condition"},
		{"target rules", rule + "[[allowlists]]\ntargetRules = ['token']\n" + candidate, "targetRules"},
		{"rule path", rule + "path = '^src/'\n[[rules.allowlists]]\n" + candidate, "rule path"},
		{"duplicate rules", rule + "[[rules.allowlists]]\n" + candidate + rule, "duplicate rule"},
		{"mixed global spellings", "[allowlist]\n" + candidate + "[[allowlists]]\nregexes = ['public']\n", "legacy and current"},
		{"mixed rule spellings", rule + "[rules.allowlist]\nregexes = ['public']\n[[rules.allowlists]]\n" + candidate, "legacy and current"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := measure(t, policyfiles.GitleaksGeneric(), fixture{files: map[string]string{".gitleaks.toml": gitleaksBase + tc.config}})
			if !model.IsOutsideSubset(res.Status) || len(res.Findings) != 0 || !strings.HasPrefix(res.Status.Reason, ".gitleaks.toml:") || !strings.Contains(res.Status.Reason, tc.construct) {
				t.Fatalf("conditional allowlist = %+v", res)
			}
			near := strings.ReplaceAll(tc.config, candidate, "paths = ['^docs/']\n")
			res = measure(t, policyfiles.GitleaksGeneric(), fixture{files: map[string]string{".gitleaks.toml": gitleaksBase + near}})
			if res.Status.Kind != model.StatusMeasured || len(res.Findings) != 0 {
				t.Fatalf("unrelated allowlist = %+v", res)
			}
		})
	}
}

func TestGitleaksGenericKeepsProvablePathMatches(t *testing.T) {
	t.Parallel()
	for _, config := range []string{
		"[[allowlists]]\ncondition = 'AND'\npaths = ['^gen/']\n",
		"[[allowlists]]\ncondition = 'OR'\nregexes = ['public']\npaths = ['^gen/']\n",
		"[[rules]]\nid = 'token'\nregex = 'tok_[a-z0-9]{16}'\n[rules.allowlist]\npaths = ['^gen/']\n",
		"[[ALLOWLISTS]]\nPATHS = ['^gen/']\n",
		"[[allowlists]]\npaths = ['^sqlcgen/']\n",
	} {
		res := measure(t, policyfiles.GitleaksGeneric(), fixture{files: map[string]string{".gitleaks.toml": gitleaksBase + config}})
		if res.Status.Kind != model.StatusMeasured || !slices.Equal(codes(res), []string{"gitleaks-generated-allowlist"}) || res.Findings[0].Line != 0 {
			t.Errorf("provable path = %+v", res)
		}
	}
}

func TestGitleaksGenericNamesInheritedRulePaths(t *testing.T) {
	t.Parallel()
	for _, id := range []string{
		"freemius-secret-key", "hashicorp-tf-password", "kubernetes-secret-yaml", "nuget-config-password", "pkcs12-file",
	} {
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			const allowlist = "[[rules.allowlists]]\npaths = ['^gen/']\n"
			rule := "[[rules]]\nid = '" + id + "'\n"
			assertInheritedRuleLimit(t, rule+allowlist)
			config := rule + "regex = 'tok_[a-z0-9]{16}'\nkeywords = ['tok_']\n" + allowlist
			assertInheritedRuleLimit(t, config)
			res := measure(t, policyfiles.GitleaksGeneric(), fixture{files: map[string]string{".gitleaks.toml": config}})
			if res.Status.Kind != model.StatusMeasured || !slices.Equal(codes(res), []string{"gitleaks-generated-allowlist"}) {
				t.Fatalf("standalone rule = %+v", res)
			}
		})
	}
}

func assertInheritedRuleLimit(t *testing.T, config string) {
	t.Helper()
	res := measure(t, policyfiles.GitleaksGeneric(), fixture{files: map[string]string{".gitleaks.toml": gitleaksBase + config}})
	if !model.IsOutsideSubset(res.Status) || len(res.Findings) != 0 || !strings.Contains(res.Status.Reason, "inherited rule path") {
		t.Fatalf("inherited rule = %+v", res)
	}
	near := strings.ReplaceAll(config, "^gen/", "^docs/")
	res = measure(t, policyfiles.GitleaksGeneric(), fixture{files: map[string]string{".gitleaks.toml": gitleaksBase + near}})
	if res.Status.Kind != model.StatusMeasured || len(res.Findings) != 0 {
		t.Fatalf("unrelated path = %+v", res)
	}
}
