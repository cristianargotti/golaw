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
// digest, as the current bundle or as the stored copy of the pinned
// version: it answers the stamp LocateProfile found, and its refusal.
func VerifyProfile(configDir string, ref ProfileRef) (Stamp, error) {
	located, err := LocateProfile(configDir, ref)
	if err != nil {
		return Stamp{}, err
	}
	return located.Stamp, nil
}

// StoredStampPath is the stamp of the stored copy of a version (D3),
// <config>/profiles/<name>@<version>/profile.yml, beside StampPath: the
// store keeps every installed version, so a repository pinned at an
// older one still loads after a newer install.
func StoredStampPath(configDir, name, version string) string {
	return filepath.Join(configDir, profilesDir, name+"@"+version, StampFile)
}

// Located is where the pinned bundle was found: the current bundle of the
// name, or its stored copy at the pinned version.
type Located struct {
	Dir    string // the bundle directory the stamp was read from
	Stamp  Stamp
	Stored bool // the stored copy answered, the current bundle differs or is absent
}

// LocateProfile reads the current stamp first and the stored stamp of the
// pinned version when the current one is absent or differs on version or
// digest (D3). A missing bundle, or one that matches on neither, is
// ErrPolicy naming the field and the exact install command (A19, D8); the
// answer of a refusal still carries the bundle found, the zero value when
// nothing is installed under the name, so doctor tells the cases apart.
func LocateProfile(configDir string, ref ProfileRef) (Located, error) {
	current, err := stampIn(StampPath(configDir, ref.Name), false)
	absent := errors.Is(err, fs.ErrNotExist)
	if err != nil && !absent {
		return Located{}, err
	}
	field := ""
	if !absent {
		if field = differs(ref, current.Stamp); field == "" {
			return current, nil
		}
		if field != fieldVersion && field != fieldDigest {
			return current, mismatch(ref, field, current.Stamp)
		}
	}
	stored, err := stampIn(StoredStampPath(configDir, ref.Name, ref.Version), true)
	switch {
	case err == nil:
		if f := differs(ref, stored.Stamp); f != "" {
			return stored, mismatch(ref, f, stored.Stamp)
		}
		return stored, nil
	case !errors.Is(err, fs.ErrNotExist):
		return Located{}, err
	case absent:
		return Located{}, fmt.Errorf("%w: profile %s is not installed: %s", model.ErrPolicy, ref.Name, installCommand(ref))
	}
	return current, mismatch(ref, field, current.Stamp)
}

// The fields of a pin a stamp may differ on, in the order they are judged.
const (
	fieldVersion = "version"
	fieldDigest  = "digest"
)

// stampIn reads the stamp at a path into the answer of its directory.
func stampIn(path string, stored bool) (Located, error) {
	s, err := LoadStamp(path)
	if err != nil {
		return Located{}, err
	}
	return Located{Dir: filepath.Dir(path), Stamp: s, Stored: stored}, nil
}

// differs names the first field of the pin a stamp does not carry, empty
// when it carries all four.
func differs(ref ProfileRef, s Stamp) string {
	for _, pair := range [][3]string{
		{"name", ref.Name, s.Name},
		{"source", ref.Source, s.Source},
		{fieldVersion, ref.Version, s.Version},
		{fieldDigest, ref.Digest, s.Digest},
	} {
		if pair[1] != pair[2] {
			return pair[0]
		}
	}
	return ""
}

// mismatch is the refusal of a field that differs: the pinned value, the
// installed one and the command that installs the pin (A19).
func mismatch(ref ProfileRef, field string, s Stamp) error {
	pinned := map[string][2]string{
		"name": {ref.Name, s.Name}, "source": {ref.Source, s.Source},
		fieldVersion: {ref.Version, s.Version}, fieldDigest: {ref.Digest, s.Digest},
	}[field]
	return fmt.Errorf("%w: profile %s: %s is %s in %s and %s in the installed bundle: %s",
		model.ErrPolicy, ref.Name, field, pinned[0], FileName, pinned[1], installCommand(ref))
}

// installCommand is the command that installs the pinned bundle.
func installCommand(ref ProfileRef) string {
	return "run golaw profile install " + ref.Source + "@" + ref.Version
}
