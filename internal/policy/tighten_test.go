package policy_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

func TestCheckTighteningAccepts(t *testing.T) {
	t.Parallel()
	inForce := decode(t, fullPolicy)
	accepted := map[string]string{
		"same":             fullPolicy,
		"lower sizes":      strings.Replace(fullPolicy, "file_lines: 250", "file_lines: 200", 1),
		"higher coverage":  strings.Replace(fullPolicy, "total: 85", "total: 90", 1),
		"fewer exemptions": strings.Replace(fullPolicy, "file_exemptions: [internal/legacy/old.go]", "file_exemptions: []", 1),
		"more tools":       strings.Replace(fullPolicy, "tools: [acme-tool, other]", "tools: [acme-tool, other, third]", 1),
		"fewer paths":      strings.Replace(fullPolicy, "allow_paths: [agents/, docs/]", "allow_paths: [agents/]", 1),
		"more caps":        strings.Replace(fullPolicy, "capabilities: [go, cli]", "capabilities: [go, cli, api]", 1),
		"no waiver":        strings.Replace(fullPolicy, "  - {id: GO-2026-6603, until: 2026-11-01, record: docs/adr/0007-go-1-27-2.md}\n", "", 1),
		"profile bump":     strings.Replace(fullPolicy, "version: 0.3.0", "version: 0.4.0", 1),
		"new pin":          strings.Replace(fullPolicy, "kit: {version: 0.1.0}", "kit: {version: 0.2.0}", 1),
		"language":         strings.Replace(fullPolicy, "language: en-US", "language: pt-BR", 1),
		"shell off":        strings.Replace(fullPolicy, "deny_push: true", "deny_push: false", 1),
		"ladder":           strings.Replace(fullPolicy, "pre_push: false", "pre_push: true", 1),
		"option down":      strings.Replace(fullPolicy, "limit: 3", "limit: 2", 1),
		"option grow":      strings.Replace(fullPolicy, "names: [a, b]", "names: [a, b, c]", 1),
		"option switch":    strings.Replace(fullPolicy, "strict: true", "strict: false", 1),
	}
	for name, text := range accepted {
		if err := policy.CheckTightening(inForce, decode(t, text), schemas()); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestCheckTighteningRefuses(t *testing.T) {
	t.Parallel()
	inForce := decode(t, fullPolicy)
	refused := map[string]struct{ text, want string }{
		"sizes up":        {strings.Replace(fullPolicy, "file_lines: 250", "file_lines: 260", 1), "checks.sizes.file_lines: 250 to 260"},
		"functions up":    {strings.Replace(fullPolicy, "function_lines: 30", "function_lines: 40", 1), "checks.sizes.function_lines"},
		"coverage down":   {strings.Replace(fullPolicy, "total: 85", "total: 80", 1), "checks.coverage.total: 85 to 80"},
		"statements up":   {strings.Replace(fullPolicy, "min_statements: 10", "min_statements: 20", 1), "checks.coverage.min_statements"},
		"more exemptions": {strings.Replace(fullPolicy, "file_exemptions: [internal/legacy/old.go]", "file_exemptions: [internal/legacy/old.go, b.go]", 1), "checks.coverage.file_exemptions"},
		"fewer tools":     {strings.Replace(fullPolicy, "tools: [acme-tool, other]", "tools: [acme-tool]", 1), "checks.terms.tools"},
		"more paths":      {strings.Replace(fullPolicy, "allow_paths: [agents/, docs/]", "allow_paths: [agents/, docs/, cmd/]", 1), "checks.terms.allow_paths"},
		"more exclusions": {strings.Replace(fullPolicy, "linters: [funlen, mnd]", "linters: [funlen, mnd, dupl]", 1), "checks.lint.allowed_exclusions"},
		"fewer caps":      {strings.Replace(fullPolicy, "capabilities: [go, cli]", "capabilities: [go]", 1), "capabilities"},
		"new waiver":      {strings.Replace(fullPolicy, "id: GO-2026-6603", "id: GO-2026-9999", 1), "vulnerability_waivers"},
		"new exception":   {strings.Replace(fullPolicy, "path: docs/old.md", "path: docs/new.md", 1), "exceptions"},
		"profile gone":    {strings.Replace(fullPolicy, "profile: {name: acme, source: https://example.com/acme.git, version: 0.3.0, digest: \"sha256:"+acmeDigest+"\"}\n", "", 1), "profile.name"},
		"profile swapped": {strings.Replace(fullPolicy, "source: https://example.com/acme.git", "source: https://example.com/evil.git", 1), "profile.source"},
		"option up":       {strings.Replace(fullPolicy, "limit: 3", "limit: 4", 1), "checks.text.tool-terms.limit"},
		"option shrink":   {strings.Replace(fullPolicy, "names: [a, b]", "names: [a]", 1), "checks.text.tool-terms.names"},
		"option absent":   {strings.Replace(fullPolicy, "limit: 3, ", "", 1), "checks.text.tool-terms.limit: 3 to 5"},
		"invalid":         {strings.Replace(fullPolicy, "golaw: 1", "golaw: 2", 1), "golaw: 2"},
	}
	for name, c := range refused {
		err := policy.CheckTightening(inForce, decode(t, c.text), schemas())
		if !errors.Is(err, model.ErrPolicy) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want ErrPolicy mentioning %q", name, err, c.want)
		}
	}
}

func TestCheckTighteningFromCore(t *testing.T) {
	t.Parallel()
	minimal := decode(t, minimalPolicy)
	if err := policy.CheckTightening(minimal, decode(t, fullPolicy), schemas()); err == nil {
		t.Error("seeding exemptions over a policy without them is a weakening")
	}
	tighter := minimalPolicy + "checks: {sizes: {file_lines: 100}, terms: {tools: [x]}}\ncapabilities: [go]\n"
	if err := policy.CheckTightening(minimal, decode(t, tighter), schemas()); err != nil {
		t.Errorf("adding floors and tools over the minimal policy: %v", err)
	}
}
