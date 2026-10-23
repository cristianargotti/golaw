package proof

import "github.com/cristianargotti/golaw/internal/engine"

// LoadManifests and Matrix serve the proof tests alone: no production code
// reads several manifests or runs the applicability matrix, so they stay
// unexported outside the tests of this package.
func LoadManifests(paths ...string) (Manifest, error) { return loadManifests(paths...) }

// Matrix runs the applicability matrix of the registry.
func Matrix(reg *engine.Registry) []Failure { return matrix(reg) }
