package coverage_test

import (
	"reflect"
	"testing"

	"github.com/cristianargotti/golaw/internal/coverage"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestFloorsFromUsesOnlyPresentTypedOptions(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		in   model.Options
		want coverage.Floors
	}{
		{
			"wrong-types",
			model.Options{
				"total": {Kind: model.OptionBool, Int: 99}, "package": model.StringsValue("90"),
				"file": model.BoolValue(false), "min_statements": {Kind: model.OptionBool, Int: 0},
				"file_exemptions": {Kind: model.OptionInt, Strings: []string{"value.go"}},
			},
			coverage.Floors{Total: 80, Package: 80, File: 80, MinStatements: 20, NewCode: model.DefaultCoverageNew},
		},
		{
			"one-threshold",
			model.Options{"package": model.IntValue(91)},
			coverage.Floors{Total: 80, Package: 91, File: 80, MinStatements: 20, NewCode: model.DefaultCoverageNew},
		},
		{
			"explicit-zero",
			model.Options{
				"total": model.IntValue(0), "package": model.IntValue(0),
				"file": model.IntValue(0), "min_statements": model.IntValue(0),
			},
			coverage.Floors{NewCode: model.DefaultCoverageNew},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := coverage.FloorsFrom(tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("floors = %+v", got)
			}
		})
	}
}

func TestFloorsFromDetachesThePolicyExemptions(t *testing.T) {
	t.Parallel()
	exemptions := []string{"old.go"}
	options := model.Options{"file_exemptions": model.StringsValue(exemptions...)}
	floors := coverage.FloorsFrom(options)
	exemptions[0] = "new.go"
	if !reflect.DeepEqual(floors.Exemptions, []string{"old.go"}) {
		t.Fatalf("floors changed with policy: %+v", floors)
	}
}
