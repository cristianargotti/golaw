package profile

import (
	"os"
	"path/filepath"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// Bundle is a bundle read whole and verified.
type Bundle struct {
	Dir   string
	Stamp policy.Stamp
	Data  model.ProfileData // Name, Version and Digest filled from the stamp
}

// Read reads a bundle directory without a pin: the stamp through
// policy.LoadStamp, the digest recomputed and compared with the stamp
// (ErrDigest naming both), rules.yml decoded and validated; a directory
// without rules.yml is ErrBundle.
func Read(dir string) (*Bundle, error) {
	stamp, err := policy.LoadStamp(filepath.Join(dir, policy.StampFile))
	if err != nil {
		return nil, err //nolint:wrapcheck // the stamp error names its path and its field
	}
	digest, err := Digest(dir)
	if err != nil {
		return nil, err
	}
	if digest != stamp.Digest {
		return nil, digestf("profile %s: digest %s of the installed bundle differs from its stamp %s: run golaw profile install %s@%s",
			stamp.Name, digest, stamp.Digest, stamp.Source, stamp.Version)
	}
	data, err := readRules(dir)
	if err != nil {
		return nil, bundlef("profile %s: %v", stamp.Name, err)
	}
	data.Name, data.Version, data.Digest = stamp.Name, stamp.Version, stamp.Digest
	return &Bundle{Dir: dir, Stamp: stamp, Data: data}, nil
}

// readRules reads, decodes and validates rules.yml, the cause alone.
func readRules(dir string) (model.ProfileData, error) {
	raw, err := os.ReadFile(filepath.Join(dir, RulesFile)) //nolint:gosec // the path is an entry of the bundle directory
	if err != nil {
		return model.ProfileData{}, causef("%v", err)
	}
	data, err := decode(raw)
	if err != nil {
		return model.ProfileData{}, err
	}
	if err := validate(data); err != nil {
		return model.ProfileData{}, err
	}
	return data, nil
}
