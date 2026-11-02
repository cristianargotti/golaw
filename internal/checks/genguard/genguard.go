package genguard

import "github.com/cristianargotti/golaw/internal/model"

// Checks lists the gen family in report order; each check is appended in
// the same change as its proof entry, so the catalog and
// internal/proof/gen.yml never disagree.
func Checks() []model.Check {
	return []model.Check{GeneratedWrite(), ManifestWrite(), HandwrittenReason(), Handwritten()}
}
