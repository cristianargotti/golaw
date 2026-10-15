package model_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestOptionSchemaValidate(t *testing.T) {
	t.Parallel()
	good := model.OptionSchema{
		{Name: "limit", Kind: model.OptionInt, Tighten: model.TightenDown, Default: model.IntValue(40)},
		{Name: "strict", Kind: model.OptionBool, Tighten: model.TightenNone, Default: model.BoolValue(false)},
		{Name: "paths", Kind: model.OptionStrings, Tighten: model.TightenShrink, Default: model.StringsValue("a", "b")},
	}
	if err := good.Validate(); err != nil {
		t.Fatalf("a valid schema is refused: %v", err)
	}
	bad := map[string]model.OptionSchema{
		"no name":        {{Name: "", Kind: model.OptionInt, Tighten: model.TightenNone, Default: model.IntValue(1)}},
		"twice":          {good[0], good[0]},
		"invalid kind":   {{Name: "x", Kind: 0, Tighten: model.TightenNone, Default: model.IntValue(1)}},
		"invalid direct": {{Name: "x", Kind: model.OptionInt, Tighten: 0, Default: model.IntValue(1)}},
		"default kind":   {{Name: "x", Kind: model.OptionInt, Tighten: model.TightenNone, Default: model.BoolValue(true)}},
	}
	for name, schema := range bad {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := schema.Validate(); !errors.Is(err, model.ErrInvalidSpec) {
				t.Fatalf("Validate() = %v, want ErrInvalidSpec", err)
			}
		})
	}
	defaults := good.Defaults()
	if v, ok := defaults.Int("limit"); !ok || v != 40 {
		t.Errorf("Defaults().Int(limit) = %d, %v", v, ok)
	}
}

func TestOptionsAccessors(t *testing.T) {
	t.Parallel()
	o := model.Options{
		"limit":  model.IntValue(7),
		"strict": model.BoolValue(true),
		"paths":  model.StringsValue("docs/", "agents/"),
	}
	if v, ok := o.Int("limit"); !ok || v != 7 {
		t.Errorf("Int(limit) = %d, %v", v, ok)
	}
	if v, ok := o.Bool("strict"); !ok || !v {
		t.Errorf("Bool(strict) = %v, %v", v, ok)
	}
	if v, ok := o.Strings("paths"); !ok || !slices.Equal(v, []string{"docs/", "agents/"}) {
		t.Errorf("Strings(paths) = %v, %v", v, ok)
	}
	if _, ok := o.Int("strict"); ok {
		t.Error("Int(strict) must refuse a bool")
	}
	if _, ok := o.Bool("missing"); ok {
		t.Error("Bool(missing) must be absent")
	}
	if _, ok := o.Strings("limit"); ok {
		t.Error("Strings(limit) must refuse an int")
	}
	if !model.OptionInt.Valid() || model.OptionKind(0).Valid() || model.OptionKind(4).Valid() {
		t.Error("OptionKind.Valid is wrong")
	}
	if !model.TightenGrow.Valid() || model.Tighten(0).Valid() || model.Tighten(6).Valid() {
		t.Error("Tighten.Valid is wrong")
	}
}
