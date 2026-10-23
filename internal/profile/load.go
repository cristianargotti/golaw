package profile

import (
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// Load reads the bundle golaw.yml names (D6): policy.LocateProfile first,
// which finds the current bundle or the stored copy of the pinned version
// and refuses with the exact install command (A19), then Read of that
// directory, which recomputes the digest the located stamp carries, so
// the data answered is the data the pin names. It is the
// policy.BundleLoader cmd binds; nothing is ever answered silently.
func Load(configDir string, ref policy.ProfileRef) (*model.ProfileData, error) {
	located, err := policy.LocateProfile(configDir, ref)
	if err != nil {
		return nil, err //nolint:wrapcheck // the A19 error is the whole reason (D8)
	}
	b, err := Read(located.Dir)
	if err != nil {
		return nil, err
	}
	return &b.Data, nil
}
