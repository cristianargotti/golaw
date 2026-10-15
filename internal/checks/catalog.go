// Package checks composes every check family into the production catalog
// the binary dispatches and the proof test verifies. A family joins the
// list in the task that adds its first check, so the catalog and the proof
// manifest grow together.
package checks

import (
	"fmt"

	"github.com/cristianargotti/golaw/internal/checks/paths"
	"github.com/cristianargotti/golaw/internal/checks/text"
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

// Family builds the checks of one family in report order.
type Family func() []model.Check

// Catalog lists every check the binary ships, family by family.
func Catalog() []model.Check {
	return concat(families()...)
}

// families are the constructors of the check families, in report order.
func families() []Family {
	return []Family{text.Checks, paths.Checks}
}

// concat joins the families into one list, keeping their order.
func concat(families ...Family) []model.Check {
	all := make([]model.Check, 0, len(families))
	for _, family := range families {
		all = append(all, family()...)
	}
	return all
}

// Registry builds the production registry from the catalog; the proof test
// passes engine.WithProofs with the manifest names.
func Registry(opts ...engine.BuildOption) (*engine.Registry, error) {
	return Build(Catalog(), opts...)
}

// Build registers the checks in order and freezes them.
func Build(checks []model.Check, opts ...engine.BuildOption) (*engine.Registry, error) {
	b := engine.New()
	for _, c := range checks {
		if err := b.Register(c); err != nil {
			return nil, fmt.Errorf("catalog: %w", err)
		}
	}
	reg, err := b.Build(opts...)
	if err != nil {
		return nil, fmt.Errorf("catalog: %w", err)
	}
	return reg, nil
}
