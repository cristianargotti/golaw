package coverage

import (
	"slices"

	"github.com/cristianargotti/golaw/internal/model"
)

// FloorsFrom preserves defaults for absent options and detaches policy-owned slices.
func FloorsFrom(o model.Options) Floors {
	floors := DefaultFloors()
	floors.NewCode = model.DefaultCoverageNew
	if value, ok := o.Int("total"); ok {
		floors.Total = float64(value)
	}
	if value, ok := o.Int("package"); ok {
		floors.Package = float64(value)
	}
	if value, ok := o.Int("file"); ok {
		floors.File = float64(value)
	}
	if value, ok := o.Int("min_statements"); ok {
		floors.MinStatements = value
	}
	if value, ok := o.Int("new_code"); ok {
		floors.NewCode = value
	}
	if value, ok := o.Strings("file_exemptions"); ok {
		floors.Exemptions = slices.Clone(value)
	}
	return floors
}
