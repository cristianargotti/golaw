package policy

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/model"
)

// Decode parses golaw.yml strictly: an unknown key, a duplicate key, a
// value of another type, an empty document or a second document is an
// error. Nothing is defaulted here; Validate judges the values.
func Decode(data []byte) (File, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var f File
	if err := dec.Decode(&f); err != nil {
		if errors.Is(err, io.EOF) {
			return File{}, fmt.Errorf("%w: %s is empty", model.ErrPolicy, FileName)
		}
		return File{}, fmt.Errorf("%w: %s: %w", model.ErrPolicy, FileName, err)
	}
	var extra File
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return File{}, fmt.Errorf("%w: %s holds more than one document", model.ErrPolicy, FileName)
	}
	return f, nil
}

// checkPin compares the kit pin with this binary before the strict
// decode, so a policy written by another kit version names the version
// to install instead of a key this binary does not know (A10). A file
// whose pin cannot be read is left to Decode to report.
func checkPin(data []byte, version string) error {
	pin, ok := pinOf(data)
	if version == "" || !ok || pin == version {
		return nil
	}
	return fmt.Errorf("%w: %s pins kit version %s and this binary is %s: install the pinned version or run init --update",
		model.ErrPolicy, FileName, pin, version)
}

// pinOf reads only the kit pin, ignoring every other key.
func pinOf(data []byte) (string, bool) {
	var f struct {
		Kit Kit `yaml:"kit"`
	}
	if yaml.Unmarshal(data, &f) != nil {
		return "", false
	}
	return f.Kit.Version, true
}
