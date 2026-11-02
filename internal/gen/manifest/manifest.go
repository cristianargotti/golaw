package manifest

import (
	"bytes"
	"errors"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/source"
)

// Path is the slash path of the manifest from the repository root.
const Path = ".golaw/gen.yml"

// Schema is the value of the golaw-gen key this kit reads and writes.
const Schema = 1

// Manifest is the whole of .golaw/gen.yml. It holds no output hash and no
// hole state: drift is found by rendering again and holes by reading the
// owned sources, so the file changes only when a kind is added or changed.
type Manifest struct {
	Schema  int     `yaml:"golaw-gen"`
	Support Support `yaml:"support"`
	Entries []Entry `yaml:"entries"`
}

// Support is the shared support package every generated part calls.
type Support struct {
	Path     string `yaml:"path"`
	Template string `yaml:"template"`
}

// Contract is one file an entry is generated from; Symbol names the
// interface when the contract is a port and stays empty otherwise.
type Contract struct {
	Path   string `yaml:"path"`
	Symbol string `yaml:"symbol,omitempty"`
}

// Entry is one generated kind instance, keyed by kind and name.
type Entry struct {
	Kind        string     `yaml:"kind"`
	Template    string     `yaml:"template"`
	Name        string     `yaml:"name"`
	Contracts   []Contract `yaml:"contracts,omitempty"`
	Ports       []string   `yaml:"ports,omitempty"`
	In          string     `yaml:"in,omitempty"`
	Out         string     `yaml:"out,omitempty"`
	Usecases    []string   `yaml:"usecases,omitempty"`
	Regenerable []string   `yaml:"regenerable,omitempty"`
	Owned       []string   `yaml:"owned,omitempty"`
}

// kindShape keeps kinds to the lower case hyphenated form the kind constants
// use, so a typo in a hand edit is refused instead of becoming a new kind.
var kindShape = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)

// Decode reads the manifest strictly: an unknown or duplicate key, a second
// document or any value Validate refuses is ErrManifest.
func Decode(data []byte) (Manifest, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		if errors.Is(err, io.EOF) {
			return Manifest{}, invalid("%s is empty", Path)
		}
		return Manifest{}, invalid("%s: %v", Path, err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return Manifest{}, invalid("%s holds more than one document", Path)
	}
	if err := m.Validate(); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// Encode writes the manifest in one deterministic form, so two runs of the
// CLI over the same entries write the same bytes and a merge stays an append.
func (m Manifest) Encode() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(m); err != nil {
		return nil, invalid("encode: %v", err)
	}
	if err := enc.Close(); err != nil {
		return nil, invalid("encode: %v", err)
	}
	return buf.Bytes(), nil
}

// Validate applies the checks of Decode to a value, so a manifest built in
// memory is held to what a read one is.
func (m Manifest) Validate() error {
	if m.Schema != Schema {
		return invalid("golaw-gen is %d, this kit reads %d", m.Schema, Schema)
	}
	if why := cleanPath(m.Support.Path); why != "" {
		return invalid("support.path: %s", why)
	}
	if !versioned(m.Support.Template, "support") {
		return invalid("support.template %q is not support@<n>", m.Support.Template)
	}
	for i := range m.Entries {
		e := &m.Entries[i]
		if why := e.validate(); why != "" {
			return invalid("entries[%d] %s: %s", i, e.Name, why)
		}
		if i == 0 {
			continue
		}
		switch order := compare(m.Entries[i-1], *e); {
		case order == 0:
			return invalid("entries[%d] %s: repeats kind %s and name %s", i, e.Name, e.Kind, e.Name)
		case order > 0:
			return invalid("entries[%d] %s: entries are not sorted by kind then name", i, e.Name)
		}
	}
	return nil
}

// validate judges one entry alone and answers why it is refused, or the
// empty string; order and repetition need the whole list.
func (e Entry) validate() string {
	if !kindShape.MatchString(e.Kind) {
		return "kind " + strconv.Quote(e.Kind) + " is not lower case with hyphens"
	}
	if area, typ, ok := strings.Cut(e.Name, "."); !ok || area == "" || typ == "" || strings.Contains(typ, ".") {
		return "name " + strconv.Quote(e.Name) + " is not <area>.<Type>"
	}
	if !versioned(e.Template, e.Kind) {
		return "template " + e.Template + " is not of kind " + e.Kind
	}
	paths := slices.Concat(e.Regenerable, e.Owned)
	for _, c := range e.Contracts {
		paths = append(paths, c.Path)
	}
	for _, p := range paths {
		if why := cleanPath(p); why != "" {
			return why
		}
	}
	return ""
}

// cleanPath accepts only the form source.CleanRel answers, so two spellings
// of one file never become two keys of Owner and Regenerable.
func cleanPath(rel string) string {
	cleaned, err := source.CleanRel(rel)
	if err != nil {
		return err.Error()
	}
	if cleaned != rel {
		return "path " + strconv.Quote(rel) + " is not clean, write " + cleaned
	}
	return ""
}

// versioned reports whether template is kind@n with n a positive integer in
// its canonical spelling.
func versioned(template, kind string) bool {
	k, n, ok := strings.Cut(template, "@")
	if !ok || k != kind {
		return false
	}
	v, err := strconv.Atoi(n)
	return err == nil && v >= 1 && strconv.Itoa(v) == n
}

// compare orders entries by kind then name, the order the CLI writes.
func compare(a, b Entry) int {
	if c := strings.Compare(a.Kind, b.Kind); c != 0 {
		return c
	}
	return strings.Compare(a.Name, b.Name)
}
