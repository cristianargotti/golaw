package assets

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"

	"gopkg.in/yaml.v3"
)

// Ownership keeps update decisions separate from template contents.
type Ownership string

// These classes let init preserve developer edits during updates.
const (
	OwnedByKit       Ownership = "kit"
	OwnedMerged      Ownership = "merged"
	OwnedByDeveloper Ownership = "developer"
)

// Template retains the conditions and ownership needed before rendering.
type Template struct {
	Source      string    `yaml:"source"`
	Destination string    `yaml:"destination"`
	Ownership   Ownership `yaml:"ownership"`
	When        []string  `yaml:"when"`
}

type templateManifest struct {
	Templates []*templateFields `yaml:"templates"`
}

// Pointers preserve null entries that YAML otherwise drops from value slices.
type templateFields struct {
	Source      string    `yaml:"source"`
	Destination string    `yaml:"destination"`
	Ownership   Ownership `yaml:"ownership"`
	When        []*string `yaml:"when"`
}

// Templates rejects an invalid manifest before any template can be applied.
func Templates(source fs.FS) ([]Template, error) {
	data, err := fs.ReadFile(source, "templates/manifest.yml")
	if err != nil {
		return nil, fmt.Errorf("%w: read template manifest: %w", ErrManifest, err)
	}
	entries, err := decodeTemplates(data)
	if err != nil {
		return nil, err
	}
	if err := validateTemplates(source, entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func decodeTemplates(data []byte) ([]Template, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var value templateManifest
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("%w: decode template manifest: %w", ErrManifest, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: template manifest must contain one document", ErrManifest)
	}
	if value.Templates == nil {
		return nil, fmt.Errorf("%w: templates must be a sequence", ErrManifest)
	}
	entries := make([]Template, 0, len(value.Templates))
	for _, entry := range value.Templates {
		decoded, err := templateValue(entry)
		if err != nil {
			return nil, err
		}
		entries = append(entries, decoded)
	}
	return entries, nil
}
