package policy_test

import (
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// schemas declares the options of the example check, one per kind and
// direction, the way the catalog publishes them to cmd.
func schemas() policy.Schemas {
	return policy.Schemas{"text.tool-terms": model.OptionSchema{
		{Name: "strict", Kind: model.OptionBool, Tighten: model.TightenNone, Default: model.BoolValue(false)},
		{Name: "limit", Kind: model.OptionInt, Tighten: model.TightenDown, Default: model.IntValue(5)},
		{Name: "names", Kind: model.OptionStrings, Tighten: model.TightenGrow, Default: model.StringsValue()},
	}}
}
