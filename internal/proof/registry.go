package proof

import (
	"fmt"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

// Registry builds the registry the proof verifies, refusing at build time a
// check whose proof pair the manifest does not name, so a check never enters
// the proven catalog without its red and near-miss fixtures.
func Registry(checks []model.Check, m Manifest) (*engine.Registry, error) {
	b := engine.New()
	for _, c := range checks {
		if err := b.Register(c); err != nil {
			return nil, fmt.Errorf("proof: register %s: %w", c.Spec().ID, err)
		}
	}
	reg, err := b.Build(engine.WithProofs(m.Names()))
	if err != nil {
		return nil, fmt.Errorf("proof: build the registry: %w", err)
	}
	return reg, nil
}
