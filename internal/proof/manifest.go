// Package proof binds every registered check to a red fixture and a
// near-miss fixture through manifest.yml and proves both directions of
// section 10.2 by wrapping registry values: the deletion pass and the
// over-fire pass. Tests use it; nothing in the binary imports it.
package proof

import (
	"bytes"
	"fmt"
	"os"
	"slices"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/model"
)

// Entry binds one check id to its fixtures. Red and Near are directories
// relative to the manifest, holding files at their in-tree paths; an event
// check holds event.json beside them; a file whose name ends in .esc holds
// Go unicode escapes and is decoded while it is loaded, so prohibited bytes
// never sit in the repository.
type Entry struct {
	ID      string `yaml:"id"`                // the check id, also the proof key
	Code    string `yaml:"code"`              // the finding code the red fixture must yield
	Red     string `yaml:"red"`               // the directory of the red fixture
	Near    string `yaml:"near"`              // the directory of the near-miss fixture
	Profile string `yaml:"profile,omitempty"` // the profile the fixtures run under, empty for core only
}

// Manifest is manifest.yml decoded.
type Manifest struct {
	Checks []Entry `yaml:"checks"`
}

// LoadManifest reads and validates manifest.yml strictly: an unknown key,
// a repeated id, an empty field or a malformed id or code is an error.
func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the manifest path is the test's own
	if err != nil {
		return Manifest{}, fmt.Errorf("%w: %w", ErrManifest, err)
	}
	var m Manifest
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("%w: %s: %w", ErrManifest, path, err)
	}
	if err := m.validate(); err != nil {
		return Manifest{}, fmt.Errorf("%s: %w", path, err)
	}
	return m, nil
}

// Names returns the check ids of the manifest, sorted.
func (m Manifest) Names() []string {
	names := make([]string, 0, len(m.Checks))
	for _, e := range m.Checks {
		names = append(names, e.ID)
	}
	slices.Sort(names)
	return names
}

// Lookup finds the entry of a check id.
func (m Manifest) Lookup(id string) (Entry, bool) {
	for _, e := range m.Checks {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}

// validate refuses an entry that breaks the format.
func (m Manifest) validate() error {
	seen := make(map[string]bool, len(m.Checks))
	for _, e := range m.Checks {
		switch {
		case !model.ValidID(e.ID):
			return fmt.Errorf("%w: id %q is not family.name", ErrManifest, e.ID)
		case seen[e.ID]:
			return fmt.Errorf("%w: id %q appears twice", ErrManifest, e.ID)
		case !model.ValidCode(e.Code):
			return fmt.Errorf("%w: %s: code %q is not kebab-case", ErrManifest, e.ID, e.Code)
		case e.Red == "" || e.Near == "":
			return fmt.Errorf("%w: %s: red and near fixtures are both required", ErrManifest, e.ID)
		case e.Profile != "" && !model.ValidCode(e.Profile):
			return fmt.Errorf("%w: %s: profile %q is not a lower case name", ErrManifest, e.ID, e.Profile)
		}
		seen[e.ID] = true
	}
	return nil
}
