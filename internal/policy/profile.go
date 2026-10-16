package policy

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/model"
)

// The layout of an installed profile bundle under the config directory.
const (
	profilesDir = "profiles"
	// StampFile is the identity file at the root of a bundle.
	StampFile = "profile.yml"
)

// Stamp is profile.yml of an installed bundle: the identity the loader of
// M5 verifies before any data is read (section 5.4).
type Stamp struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Date    string `yaml:"date"`
	Source  string `yaml:"source"`
	Digest  string `yaml:"digest"`
}

// StampPath is where the stamp of a named profile lives.
func StampPath(configDir, name string) string {
	return filepath.Join(configDir, profilesDir, name, StampFile)
}

// LoadStamp reads a stamp strictly and validates every field.
func LoadStamp(path string) (Stamp, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the path is built under the config directory
	if err != nil {
		return Stamp{}, fmt.Errorf("%w: %w", model.ErrPolicy, err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var s Stamp
	if err := dec.Decode(&s); err != nil {
		return Stamp{}, fmt.Errorf("%w: %s: %w", model.ErrPolicy, path, err)
	}
	ref := ProfileRef{Name: s.Name, Source: s.Source, Version: s.Version, Digest: s.Digest}
	if err := ref.validate(); err != nil {
		return Stamp{}, fmt.Errorf("%w: %s: %w", model.ErrPolicy, path, err)
	}
	if _, err := time.Parse(dateLayout, s.Date); err != nil {
		return Stamp{}, fmt.Errorf("%w: %s: date %q is not a date like 2026-10-14", model.ErrPolicy, path, s.Date)
	}
	return s, nil
}

// VerifyProfile checks that the bundle golaw.yml names is installed under
// the config directory and matches it on name, source, version and
// digest. A missing bundle names the exact install command (A19); a
// mismatch names the field, never falls back (decision 3).
func VerifyProfile(configDir string, ref ProfileRef) (Stamp, error) {
	s, err := LoadStamp(StampPath(configDir, ref.Name))
	if errors.Is(err, fs.ErrNotExist) {
		return Stamp{}, fmt.Errorf("%w: profile %s is not installed: run golaw profile install %s@%s",
			model.ErrPolicy, ref.Name, ref.Source, ref.Version)
	}
	if err != nil {
		return Stamp{}, err
	}
	installed := ProfileRef{Name: s.Name, Source: s.Source, Version: s.Version, Digest: s.Digest}
	for _, pair := range [][3]string{
		{"name", ref.Name, installed.Name},
		{"source", ref.Source, installed.Source},
		{"version", ref.Version, installed.Version},
		{"digest", ref.Digest, installed.Digest},
	} {
		if pair[1] != pair[2] {
			return Stamp{}, fmt.Errorf("%w: profile %s: %s is %s in %s and %s in the installed bundle",
				model.ErrPolicy, ref.Name, pair[0], pair[1], FileName, pair[2])
		}
	}
	return s, nil
}
