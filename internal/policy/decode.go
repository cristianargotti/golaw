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
