package proof

import "fmt"

// LoadManifests reads several manifests as one proof, each strictly as
// LoadManifest reads it, and refuses an id that appears in two of them, so
// the completeness pass compares the whole registry with the whole proof.
// No production code reads several manifests, so it lives with the tests.
func LoadManifests(paths ...string) (Manifest, error) {
	if len(paths) == 0 {
		return Manifest{}, fmt.Errorf("%w: no manifest named", ErrManifest)
	}
	var all Manifest
	for _, path := range paths {
		m, err := LoadManifest(path)
		if err != nil {
			return Manifest{}, err
		}
		all.Checks = append(all.Checks, m.Checks...)
	}
	if err := all.validate(); err != nil {
		return Manifest{}, fmt.Errorf("%v: %w", paths, err)
	}
	return all, nil
}
