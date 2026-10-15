package policy_test

import (
	"errors"
	"maps"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestValidateAccepts(t *testing.T) {
	t.Parallel()
	for name, text := range map[string]string{"full": fullPolicy, "minimal": minimalPolicy} {
		if err := decode(t, text).Validate(schemas()); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	edge := minimalPolicy + "checks:\n  sizes: {file_lines: 300, function_lines: 1}\n  coverage: {total: 100, min_statements: 0}\n  terms: {allow_paths: [agents/]}\n"
	if err := decode(t, edge).Validate(nil); err != nil {
		t.Errorf("the floors at their bounds: %v", err)
	}
}

// invalidCase is one file Validate must refuse with the words it names.
type invalidCase struct {
	text string
	want string
}

func invalidPolicies() map[string]invalidCase {
	out := invalidHeads()
	maps.Copy(out, invalidRecords())
	return out
}

func invalidHeads() map[string]invalidCase {
	return map[string]invalidCase{
		"schema":            {"golaw: 2\nkit: {version: 0.1.0}\n", "golaw: 2"},
		"no pin":            {"golaw: 1\n", "kit.version"},
		"range pin":         {"golaw: 1\nkit: {version: ^0.1.0}\n", "kit.version"},
		"v pin":             {"golaw: 1\nkit: {version: v0.1.0}\n", "kit.version"},
		"language":          {minimalPolicy + "language: portuguese\n", "language"},
		"profile name":      {minimalPolicy + "profile: {name: Acme, source: s, version: 0.1.0, digest: sha256:" + acmeDigest + "}\n", "profile.name"},
		"profile source":    {minimalPolicy + "profile: {name: acme, version: 0.1.0, digest: sha256:" + acmeDigest + "}\n", "no source"},
		"profile version":   {minimalPolicy + "profile: {name: acme, source: s, version: latest, digest: sha256:" + acmeDigest + "}\n", "profile.version"},
		"profile digest":    {minimalPolicy + "profile: {name: acme, source: s, version: 0.1.0, digest: abc}\n", "profile.digest"},
		"layout main":       {minimalPolicy + "layout: {main: lib}\n", "layout.main"},
		"layout decisions":  {minimalPolicy + "layout: {decisions: /abs}\n", "layout.decisions"},
		"capability":        {minimalPolicy + "capabilities: [web]\n", "capabilities names unknown"},
		"capability twice":  {minimalPolicy + "capabilities: [go, go]\n", "capabilities repeats"},
		"file lines up":     {minimalPolicy + "checks: {sizes: {file_lines: 301}}\n", "checks.sizes.file_lines 301"},
		"file lines zero":   {minimalPolicy + "checks: {sizes: {file_lines: 0}}\n", "checks.sizes.file_lines 0"},
		"function lines up": {minimalPolicy + "checks: {sizes: {function_lines: 41}}\n", "checks.sizes.function_lines 41"},
		"coverage down":     {minimalPolicy + "checks: {coverage: {total: 79}}\n", "checks.coverage.total 79"},
		"coverage over":     {minimalPolicy + "checks: {coverage: {file: 101}}\n", "checks.coverage.file 101"},
		"min statements up": {minimalPolicy + "checks: {coverage: {min_statements: 21}}\n", "checks.coverage.min_statements 21"},
		"exemption path":    {minimalPolicy + "checks: {coverage: {file_exemptions: [../x.go]}}\n", "file_exemptions[0]"},
		"exemption twice":   {minimalPolicy + "checks: {coverage: {file_exemptions: [a.go, a.go]}}\n", "file_exemptions repeats"},
		"tool name":         {minimalPolicy + "checks: {terms: {tools: [Acme]}}\n", "checks.terms.tools[0]"},
		"tool twice":        {minimalPolicy + "checks: {terms: {tools: [a, a]}}\n", "checks.terms.tools repeats"},
		"allow path":        {minimalPolicy + "checks: {terms: {allow_paths: [/agents]}}\n", "checks.terms.allow_paths[0]"},
		"unclean path":      {minimalPolicy + "checks: {terms: {allow_paths: [./agents/]}}\n", "checks.terms.allow_paths[0]"},
		"lint empty":        {minimalPolicy + "checks: {lint: {allowed_exclusions: [{path: x}]}}\n", "allowed_exclusions[0]"},
		"lint linter":       {minimalPolicy + "checks: {lint: {allowed_exclusions: [{path: x, linters: [Funlen]}]}}\n", "allowed_exclusions[0].linters"},
		"option block":      {minimalPolicy + "checks: {sizez: {file_lines: 1}}\n", "checks.sizez"},
		"option unknown":    {minimalPolicy + "checks: {text.other: {limit: 1}}\n", "checks.text.other"},
		"option name":       {minimalPolicy + "checks: {text.tool-terms: {depth: 1}}\n", "checks.text.tool-terms.depth"},
		"option kind":       {minimalPolicy + "checks: {text.tool-terms: {limit: true}}\n", "checks.text.tool-terms.limit"},
		"option list":       {minimalPolicy + "checks: {text.tool-terms: {names: [1]}}\n", "not a string"},
	}
}

func invalidRecords() map[string]invalidCase {
	return map[string]invalidCase{
		"waiver id":        {minimalPolicy + "vulnerability_waivers: [{until: 2026-11-01, record: r.md}]\n", "vulnerability_waivers[0] has no id"},
		"waiver date":      {minimalPolicy + "vulnerability_waivers: [{id: X, until: soon, record: r.md}]\n", "vulnerability_waivers[0].until"},
		"waiver record":    {minimalPolicy + "vulnerability_waivers: [{id: X, until: 2026-11-01, record: /r.md}]\n", "vulnerability_waivers[0].record"},
		"exception check":  {minimalPolicy + "exceptions: [{check: dashes, path: a.go, record: r.md, until: 2026-12-31}]\n", "exceptions[0].check"},
		"exception path":   {minimalPolicy + "exceptions: [{check: text.dashes, path: \"\", record: r.md, until: 2026-12-31}]\n", "exceptions[0].path"},
		"exception record": {minimalPolicy + "exceptions: [{check: text.dashes, path: a.go, record: ../r.md, until: 2026-12-31}]\n", "exceptions[0].record"},
		"exception until":  {minimalPolicy + "exceptions: [{check: text.dashes, path: a.go, record: r.md}]\n", "exceptions[0].until"},
		"managed path":     {minimalPolicy + "managed: [{path: /x, sha256: " + strings.Repeat("ab", 32) + "}]\n", "managed[0].path"},
		"managed hash":     {minimalPolicy + "managed: [{path: x, sha256: abc}]\n", "managed[0].sha256"},
	}
}

func TestValidateRefuses(t *testing.T) {
	t.Parallel()
	for name, c := range invalidPolicies() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := decode(t, c.text).Validate(schemas())
			if !errors.Is(err, model.ErrPolicy) || !strings.Contains(err.Error(), c.want) {
				t.Errorf("Validate = %v, want ErrPolicy mentioning %q", err, c.want)
			}
		})
	}
	if err := decode(t, minimalPolicy+"checks: {text.tool-terms: {limit: 1}}\n").Validate(nil); !errors.Is(err, model.ErrPolicy) {
		t.Errorf("without schemas no check declares options: %v", err)
	}
}
