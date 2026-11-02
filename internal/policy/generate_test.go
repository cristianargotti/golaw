package policy_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// generatePolicy names the hub's layout and turns the hook regeneration off.
const generatePolicy = `golaw: 1
kit: {version: 0.1.0}
generate:
  layout:
    adapter_sql: internal/adapters/out/{package}
    handler: internal/adapters/in/{package}
  support: internal/platform/gensupport
  third_party:
    openapi_out: api/gen
ladder: {hook_regenerate: false}
`

func TestCompileResolvesTheGenerateBlockOverTheDefaults(t *testing.T) {
	t.Parallel()
	p, err := policy.Compile(decode(t, generatePolicy), now(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := model.DefaultGenerate()
	want.Support = "internal/platform/gensupport"
	want.Declared = true
	if p.Generate != want {
		t.Errorf("Generate = %+v, want %+v", p.Generate, want)
	}
	if p.Ladder.HookRegenerate {
		t.Error("ladder.hook_regenerate: false left the regeneration on")
	}
	minimal, err := policy.Compile(decode(t, minimalPolicy), now(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if minimal.Generate != model.DefaultGenerate() || !minimal.Ladder.HookRegenerate {
		t.Errorf("minimal = %+v, %+v", minimal.Generate, minimal.Ladder)
	}
}

func TestTheSQLCOutputFollowsTheAdapterLayout(t *testing.T) {
	t.Parallel()
	text := "golaw: 1\nkit: {version: 0.1.0}\ngenerate:\n  layout:\n    adapter_sql: internal/infra/{package}\n"
	p, err := policy.Compile(decode(t, text), now(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Generate.ThirdParty.SQLCOut; got != "internal/infra/postgres/sqlcgen" {
		t.Errorf("derived sqlc_out = %q, want internal/infra/postgres/sqlcgen", got)
	}
	named := text + "  third_party:\n    sqlc_out: internal/infra/db/sqlcgen\n"
	p, err = policy.Compile(decode(t, named), now(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Generate.ThirdParty.SQLCOut; got != "internal/infra/db/sqlcgen" {
		t.Errorf("named sqlc_out = %q", got)
	}
}

func TestValidateRefusesAGenerateBlockItCannotRender(t *testing.T) {
	t.Parallel()
	block := func(body string) string { return "golaw: 1\nkit: {version: 0.1.0}\ngenerate:\n" + body }
	tests := map[string]string{
		"a path that leaves the root":         block("  support: ../gensupport\n"),
		"an absolute path":                    block("  support: /internal/gensupport\n"),
		"a placeholder the key does not take": block("  layout:\n    port: internal/application/{package}\n"),
		"a placeholder in the support path":   block("  support: internal/{area}/gensupport\n"),
		"an unknown placeholder":              block("  layout:\n    wiring: cmd/{binary}\n"),
		"an output not named gen":             block("  third_party:\n    openapi_out: api/generated\n"),
		"an sqlc output not named sqlcgen":    block("  third_party:\n    sqlc_out: internal/adapters/out/postgres/queries\n"),
	}
	for name, text := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := decode(t, text)
			if err := f.Validate(nil); !errors.Is(err, model.ErrPolicy) {
				t.Errorf("Validate = %v, want ErrPolicy", err)
			}
		})
	}
	if _, err := policy.Decode([]byte(block("  layouts: {}\n"))); err == nil {
		t.Error("an unknown key of the generate block decoded")
	}
	for name, text := range map[string]string{
		"near: the defaults written out":                 block("  layout:\n    port: internal/application/{area}\n    wiring: cmd/{main}\n  third_party:\n    openapi_out: api/gen\n"),
		"near: an output directory named sqlcgen deeper": block("  third_party:\n    sqlc_out: internal/db/postgres/sqlcgen\n"),
		"near: an empty block":                           block("  {}\n"),
	} {
		if err := decode(t, text).Validate(nil); err != nil {
			t.Errorf("%s: Validate = %v, want nil", name, err)
		}
	}
}

func TestTheBlockAndTheSwitchStayOutsideTheFloors(t *testing.T) {
	t.Parallel()
	base := decode(t, minimalPolicy)
	with := decode(t, generatePolicy)
	if policy.Digest(base) != policy.Digest(with) {
		t.Error("the generate block or ladder.hook_regenerate entered the digest")
	}
	if err := policy.CheckTightening(base, with, nil); err != nil {
		t.Errorf("adding the block is not neutral: %v", err)
	}
	if err := policy.CheckTightening(with, base, nil); err != nil {
		t.Errorf("removing the block is not neutral: %v", err)
	}
}

func TestExplainNamesTheDeclaredGenerateKeys(t *testing.T) {
	t.Parallel()
	root, d := enrolled(t, generatePolicy)
	l, err := policy.Load(t.Context(), input(root, d))
	if err != nil {
		t.Fatal(err)
	}
	got := lines(policy.Explain(l))
	for key, value := range map[string]string{
		"generate.layout.adapter_sql":      "internal/adapters/out/{package}",
		"generate.layout.port":             "internal/application/{area}",
		"generate.support":                 "internal/platform/gensupport",
		"generate.third_party.sqlc_out":    "internal/adapters/out/postgres/sqlcgen",
		"generate.third_party.openapi_out": "api/gen",
		"ladder.hook_regenerate":           "false",
	} {
		if got[key].Value != value {
			t.Errorf("%s = %+v, want %q", key, got[key], value)
		}
	}
	if got["generate.layout.port"].Source != policy.SourceCore || strings.Contains(got["generate.support"].Source, "core") {
		t.Errorf("sources = %q, %q", got["generate.layout.port"].Source, got["generate.support"].Source)
	}
	plain, err := policy.Load(t.Context(), input(enrolled(t, minimalPolicy)))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := lines(policy.Explain(plain))["generate.support"]; ok {
		t.Error("a policy without the block explains generate keys")
	}
}
