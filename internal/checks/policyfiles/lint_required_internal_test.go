package policyfiles

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestRequiredLintersFollowTheCapabilities(t *testing.T) {
	t.Parallel()
	policy := func(capabilities ...string) *model.EffectivePolicy {
		p := model.CoreDefaults()
		p.Capabilities = capabilities
		return &p
	}
	core := requiredLinters(policy("go"))
	if len(core) != 81 || !slices.IsSorted(core) || len(slices.Compact(slices.Clone(core))) != len(core) {
		t.Fatalf("the core set has %d names, sorted %v", len(core), slices.IsSorted(core))
	}
	for _, name := range []string{"errcheck", "govet", "ineffassign", "staticcheck", "unused", "depguard", "forbidigo", "gocritic", "funlen", "wrapcheck"} {
		if !slices.Contains(core, name) {
			t.Errorf("the core set lacks %s", name)
		}
	}
	for _, shaped := range []string{"promlinter", "rowserrcheck", "spancheck", "sqlclosecheck", "tagliatelle", "testifylint", "unqueryvet"} {
		if slices.Contains(core, shaped) {
			t.Errorf("the core set requires %s, which a capability conditions", shaped)
		}
	}
	db := requiredLinters(policy("db", "go"))
	api := requiredLinters(policy("api", "go"))
	every := requiredLinters(policy(model.Capabilities()...))
	for _, tc := range []struct {
		got   []string
		added []string
	}{
		{db, []string{"rowserrcheck", "sqlclosecheck", "unqueryvet"}},
		{api, []string{"promlinter", "spancheck", "tagliatelle"}},
		{every, []string{"promlinter", "rowserrcheck", "spancheck", "sqlclosecheck", "tagliatelle", "unqueryvet"}},
	} {
		want := slices.Sorted(slices.Values(append(slices.Clone(core), tc.added...)))
		if !slices.Equal(tc.got, want) {
			t.Errorf("required = %v, want the core set and %v", tc.got, tc.added)
		}
	}
	if slices.Contains(every, "testifylint") {
		t.Error("testifylint judges a library no capability names and is never required")
	}
}

func TestEnabledReadsTheDefaultAndTheLists(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		defaults string
		enable   []string
		linter   string
		on       bool
		known    bool
	}{
		{defaultAll, nil, "gosec", true, true},
		{defaultStandard, nil, "errcheck", true, true},
		{defaultStandard, nil, "gosec", false, true},
		{defaultStandard, []string{"gosec"}, "gosec", true, true},
		{defaultNone, nil, "errcheck", false, true},
		{defaultNone, []string{"errcheck"}, "errcheck", true, true},
		{defaultFast, []string{"gosec"}, "gosec", true, true},
		{defaultFast, nil, "gosec", false, false},
	} {
		cfg := golangci{defaults: tc.defaults}
		for _, name := range tc.enable {
			cfg.enable = append(cfg.enable, named{name: name, line: 1})
		}
		if on, known := cfg.enabled(tc.linter); on != tc.on || known != tc.known {
			t.Errorf("%s with %v: %s = %v, %v", tc.defaults, tc.enable, tc.linter, on, known)
		}
	}
}
