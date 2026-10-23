package profile

import (
	"bytes"
	"errors"
	"io"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/model"
)

// rules is rules.yml as the file holds it (D4): the schema key and every
// section optional, each in snake case, decoded strictly before Validate.
type rules struct {
	Schema     int         `yaml:"golaw_profile"`
	Terms      []string    `yaml:"terms"`
	Paths      []string    `yaml:"paths"`
	Layout     *layout     `yaml:"layout"`
	Templates  *templates  `yaml:"templates"`
	Targets    []string    `yaml:"targets"`
	Dockerfile *dockerfile `yaml:"dockerfile"`
	Registry   *registry   `yaml:"registry"`
	Assertions []assertion `yaml:"assertions"`
	Branch     *string     `yaml:"branch"` // reserved for a later version (D19)
}

type layout struct {
	Main      string `yaml:"main"`
	Decisions string `yaml:"decisions"`
}

type templates struct {
	Exclude []string `yaml:"exclude"`
}

type dockerfile struct {
	Stages         []string  `yaml:"stages"`
	FinalImage     string    `yaml:"final_image"`
	Installs       []install `yaml:"installs"`
	PinnedInstalls bool      `yaml:"pinned_installs"`
	IgnoreComments []string  `yaml:"ignore_comments"`
}

type install struct {
	Stage    string   `yaml:"stage"`
	Packages []string `yaml:"packages"`
}

type registry struct {
	Pattern string `yaml:"pattern"`
}

type assertion struct {
	Path    string   `yaml:"path"`
	Key     []string `yaml:"key"`
	Kind    string   `yaml:"kind"`
	Equals  *string  `yaml:"equals"`
	Missing string   `yaml:"missing"`
	Files   string   `yaml:"files"`
}

// Decode parses rules.yml strictly: golaw_profile must be SchemaVersion,
// an unknown or duplicate key, a second document or a value of another
// type is ErrBundle; nothing is defaulted and Name, Version and Digest
// stay empty for the stamp to fill.
func Decode(data []byte) (model.ProfileData, error) {
	d, err := decode(data)
	if err != nil {
		return model.ProfileData{}, bundlef("%v", err)
	}
	return d, nil
}

// decode is Decode with the cause alone, so Read can name the profile.
func decode(data []byte) (model.ProfileData, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var r rules
	if err := dec.Decode(&r); err != nil {
		if errors.Is(err, io.EOF) {
			return model.ProfileData{}, causef("the file is empty")
		}
		return model.ProfileData{}, causef("%s", unknownKey(oneLine(err)))
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return model.ProfileData{}, causef("a second document is not allowed")
	}
	switch {
	case r.Schema > SchemaVersion:
		return model.ProfileData{}, causef("golaw_profile is %d and this golaw reads %d; %s", r.Schema, SchemaVersion, newerKit)
	case r.Schema != SchemaVersion:
		return model.ProfileData{}, causef("golaw_profile must be %d", SchemaVersion)
	case r.Branch != nil:
		return model.ProfileData{}, causef("branch is not supported in this version")
	}
	return r.data(), nil
}

// oneLine folds a cause yaml.v3 spreads over several lines into one, so
// the refusal stays the one golaw: line of D8.
func oneLine(err error) string {
	return strings.Join(strings.Fields(err.Error()), " ")
}

// newerKit is the remedy of a bundle this golaw cannot read because a
// newer golaw wrote its schema.
const newerKit = "a bundle written for a newer golaw needs that golaw"

// unknownKey names the two remedies of a key this golaw does not know, on
// the same line: a newer golaw for a newer bundle, or the key fixed.
func unknownKey(cause string) string {
	if !strings.Contains(cause, "not found in type") {
		return cause
	}
	return cause + "; " + newerKit + ", otherwise fix the key"
}

// data converts the file shape into the model, nothing defaulted.
func (r rules) data() model.ProfileData {
	d := model.ProfileData{Terms: r.Terms, Paths: r.Paths, Targets: r.Targets}
	if r.Layout != nil {
		d.Layout = &model.Layout{Main: r.Layout.Main, Decisions: r.Layout.Decisions}
	}
	if r.Templates != nil {
		d.Templates.Exclude = r.Templates.Exclude
	}
	if r.Dockerfile != nil {
		d.Dockerfile = r.Dockerfile.rules()
	}
	if r.Registry != nil {
		d.Registry = &model.RegistryRule{Pattern: r.Registry.Pattern}
	}
	for _, a := range r.Assertions {
		d.Assertions = append(d.Assertions, model.Assertion{
			Path: a.Path, Key: a.Key, Kind: a.Kind, Equals: a.Equals, Missing: a.Missing, Files: a.Files,
		})
	}
	return d
}

// rules converts the image section into the model.
func (f dockerfile) rules() *model.DockerfileRules {
	out := &model.DockerfileRules{
		Stages: f.Stages, FinalImage: f.FinalImage, PinnedInstalls: f.PinnedInstalls, IgnoreComments: f.IgnoreComments,
	}
	for _, i := range f.Installs {
		out.Installs = append(out.Installs, model.InstallAllowlist{Stage: i.Stage, Packages: i.Packages})
	}
	return out
}
