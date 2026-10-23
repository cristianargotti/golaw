package profile

import (
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// Load reads the bundle golaw.yml names (D6): policy.VerifyProfile first
// (the A19 errors, each naming the exact install command), then Read of
// the current bundle of the name, then the pinned digest against the one
// recomputed over the files (ErrDigest, D8), answering the data. It is
// the policy.BundleLoader cmd binds; nothing is ever answered silently.
func Load(configDir string, ref policy.ProfileRef) (*model.ProfileData, error) {
	if _, err := policy.VerifyProfile(configDir, ref); err != nil {
		return nil, err //nolint:wrapcheck // the A19 error is the whole reason (D8)
	}
	b, err := Read(Dir(configDir, ref.Name))
	if err != nil {
		return nil, err
	}
	if b.Data.Digest != ref.Digest {
		return nil, digestf("profile %s: digest %s of the installed bundle differs from the pin %s: run golaw profile install %s@%s",
			ref.Name, b.Data.Digest, ref.Digest, ref.Source, ref.Version)
	}
	return &b.Data, nil
}
