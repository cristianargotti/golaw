package thirdparty_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/thirdparty"
	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/model"
)

const converter = "package growth\n\n// Convert maps the plans of the adapters.\n// goverter:converter\ntype Convert interface {\n\tPlan(in Row) Plan\n}\n"

func enabled(capabilities ...string) *model.EffectivePolicy {
	p := model.CoreDefaults()
	p.Capabilities = capabilities
	return &p
}

func configured(t *testing.T) model.Snapshot {
	t.Helper()
	return gentest.Snapshot(t, map[string][]byte{
		"sqlc.yaml":                              []byte("version: \"2\"\n"),
		"api/codegen/server.yaml":                []byte("package: apigen\n"),
		"api/codegen/types.yaml":                 []byte("package: apigen\n"),
		"internal/application/growth/convert.go": []byte(converter),
		"internal/application/other/convert.go":  []byte("package other\n\n// Convert has no converter marker.\ntype Convert interface{}\n"),
		"internal/application/growth/ports.go":   []byte("package growth\n\n// goverter:converter in a file of another name\n"),
	})
}

func TestGeneratorsFollowTheCapabilitiesAndTheConfigurations(t *testing.T) {
	t.Parallel()
	got := thirdparty.Generators(enabled("api", "db", "go", "hexagonal"), configured(t))
	want := []thirdparty.Generator{
		{
			Name: "sqlc", Config: "sqlc.yaml", Args: []string{"generate", "-f", "sqlc.yaml"},
			Inputs: []string{"db/queries/*.sql", "db/migrations/*.sql"}, Outputs: []string{"internal/adapters/out/postgres/sqlcgen"}, Hook: true,
		},
		{
			Name: "oapi-codegen", Config: "api/codegen/server.yaml", Args: []string{"-config", "api/codegen/server.yaml", "api/openapi.yaml"},
			Inputs: []string{"api/openapi.yaml"}, Outputs: []string{"api/gen"}, Hook: true,
		},
		{
			Name: "oapi-codegen", Config: "api/codegen/types.yaml", Args: []string{"-config", "api/codegen/types.yaml", "api/openapi.yaml"},
			Inputs: []string{"api/openapi.yaml"}, Outputs: []string{"api/gen"}, Hook: true,
		},
		{
			Name: "goverter", Config: "internal/application/growth/convert.go", Args: []string{"gen", "./internal/application/growth"},
			Inputs: []string{"internal/application/growth/convert.go"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Generators = %+v\nwant         %+v", got, want)
	}
}

// Red and near rows: a generator runs only under its capability and only
// over a configuration the snapshot holds.
func TestGeneratorsStayOffWithoutTheirCapabilityOrConfiguration(t *testing.T) {
	t.Parallel()
	if got := thirdparty.Generators(enabled("go"), configured(t)); len(got) != 0 {
		t.Errorf("no capability = %+v", got)
	}
	empty := gentest.Snapshot(t, map[string][]byte{"go.mod": []byte("module example.com/slice\n")})
	if got := thirdparty.Generators(enabled("api", "db", "hexagonal"), empty); len(got) != 0 {
		t.Errorf("no configuration = %+v", got)
	}
	if got := thirdparty.Generators(nil, configured(t)); got != nil {
		t.Errorf("a nil policy = %+v", got)
	}
	custom := enabled("db")
	custom.Generate.ThirdParty.SQLCOut = "internal/adapters/outbound/postgres/sqlcgen"
	if got := thirdparty.Generators(custom, configured(t)); len(got) != 1 || got[0].Outputs[0] != custom.Generate.ThirdParty.SQLCOut {
		t.Errorf("custom sqlc_out = %+v", got)
	}
}

func TestTriggeredKeepsTheGeneratorsAChangedPathFeeds(t *testing.T) {
	t.Parallel()
	all := thirdparty.Generators(enabled("api", "db", "hexagonal"), configured(t))
	tests := map[string]struct {
		changed []string
		names   []string
	}{
		"a query":                 {[]string{"db/queries/plans.sql"}, []string{"sqlc"}},
		"a migration and the API": {[]string{"db/migrations/0002_x.sql", "api/openapi.yaml"}, []string{"sqlc", "oapi-codegen", "oapi-codegen"}},
		"a converter":             {[]string{"internal/application/growth/convert.go"}, []string{"goverter"}},
		"near: a nested query":    {[]string{"db/queries/sub/plans.sql"}, nil},
		"near: a port contract":   {[]string{"internal/application/growth/ports.go"}, nil},
		"near: nothing changed":   {nil, nil},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			triggered := thirdparty.Triggered(all, tc.changed)
			names := make([]string, 0, len(triggered))
			for _, g := range triggered {
				names = append(names, g.Name)
			}
			if !slices.Equal(names, tc.names) {
				t.Errorf("Triggered = %v, want %v", names, tc.names)
			}
		})
	}
}

// Red rows apply: every input present, or an output already generated.
// Near rows do not: an input glob that matches nothing, nothing generated.
func TestApplicableNeedsTheInputsOrAnOutput(t *testing.T) {
	t.Parallel()
	sqlc := thirdparty.Generator{Name: "sqlc", Inputs: []string{"db/queries/*.sql", "db/migrations/*.sql"}, Outputs: []string{"internal/sqlcgen"}}
	for name, tc := range map[string]struct {
		files []string
		want  bool
	}{
		"red: every input present":         {[]string{"db/queries/plans.sql", "db/migrations/0001_plans.sql"}, true},
		"red: an output already generated": {[]string{"internal/sqlcgen/db.go"}, true},
		"near: one input glob unmatched":   {[]string{"db/migrations/0001_plans.sql"}, false},
		"near: nothing at all":             {[]string{"sqlc.yaml"}, false},
		"near: a sibling of the output":    {[]string{"internal/sqlcgenx/db.go"}, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			files := map[string][]byte{}
			for _, f := range tc.files {
				files[f] = []byte("x\n")
			}
			if got := thirdparty.Applicable(gentest.Snapshot(t, files), sqlc); got != tc.want {
				t.Errorf("Applicable over %v = %v, want %v", tc.files, got, tc.want)
			}
		})
	}
}
