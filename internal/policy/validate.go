package policy

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
)

// The keys the validation, the tightening and the explanation all name.
const (
	keyCapabilities  = "capabilities"
	keyFileLines     = "checks.sizes.file_lines"
	keyFunctionLines = "checks.sizes.function_lines"
	keyAllowPaths    = "checks.terms.allow_paths"
)

// The core floors a repository policy may only tighten.
const (
	DefaultCoverage      = 80
	DefaultMinStatements = 20
	percentCap           = 100
	digestHexLength      = 64
	dateLayout           = "2006-01-02"
)

var (
	versionPattern  = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`)
	namePattern     = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	languagePattern = regexp.MustCompile(`^[a-z]{2}(-[A-Z]{2})?$`)
	digestPattern   = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	hexPattern      = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// violation is one rule a file broke; it matches model.ErrPolicy and
// prints its name first, so a hook reason and a scan error read alike.
type violation struct {
	text string
}

func (v *violation) Error() string { return model.ErrPolicy.Error() + ": " + v.text }

// Unwrap makes every violation match model.ErrPolicy.
func (v *violation) Unwrap() error { return model.ErrPolicy }

// invalid builds a violation from a format and its arguments.
func invalid(format string, args ...any) error {
	return &violation{text: fmt.Sprintf(format, args...)}
}

// Schemas maps a check id to the options it declares, as the catalog
// publishes them; the policy package validates and tightens per-check
// options against it without importing the checks.
type Schemas map[string]model.OptionSchema

// Validate refuses a decoded file that breaks the schema or weakens a core
// floor: the schema and the exact kit pin are required, the profile
// reference is complete, the language is a tag, every capability is known,
// sizes stay at or below the core defaults, coverage stays at or above
// them, every path is relative and clean, every date parses and every
// per-check option matches the schema of its check.
func (f File) Validate(schemas Schemas) error {
	for _, err := range []error{
		f.validateHead(),
		f.validateLists(),
		f.Checks.validate(schemas),
		f.validateRecords(),
	} {
		if err != nil {
			return err
		}
	}
	return nil
}

// validateHead judges the schema, the kit pin, the profile, the language
// and the layout.
func (f File) validateHead() error {
	switch {
	case f.Schema != Schema:
		return invalid("golaw: %d is not the schema this kit reads (%d)", f.Schema, Schema)
	case !versionPattern.MatchString(f.Kit.Version):
		return invalid("kit.version %q is not an exact X.Y.Z pin", f.Kit.Version)
	case f.Language != nil && !languagePattern.MatchString(*f.Language):
		return invalid("language %q is not a tag like pt-BR", *f.Language)
	}
	if err := f.Profile.validate(); err != nil {
		return err
	}
	return f.Layout.validate()
}

// validate judges a profile reference: every field is required.
func (p *ProfileRef) validate() error {
	if p == nil {
		return nil
	}
	switch {
	case !namePattern.MatchString(p.Name):
		return invalid("profile.name %q is not a lower case name", p.Name)
	case p.Source == "":
		return invalid("profile %s has no source", p.Name)
	case !versionPattern.MatchString(p.Version):
		return invalid("profile.version %q is not an exact X.Y.Z pin", p.Version)
	case !digestPattern.MatchString(p.Digest):
		return invalid("profile.digest %q is not sha256:<64 hex digits>", p.Digest)
	}
	return nil
}

// validate judges the layout: main is root or cmd, decisions a relative path.
func (l *Layout) validate() error {
	if l == nil {
		return nil
	}
	if l.Main != nil && *l.Main != "root" && *l.Main != "cmd" {
		return invalid("layout.main %q is not root or cmd", *l.Main)
	}
	if l.Decisions != nil {
		if err := relPath("layout.decisions", *l.Decisions); err != nil {
			return err
		}
	}
	return nil
}

// validateLists judges the capabilities: known and unique, then the floor
// of their implications.
func (f File) validateLists() error {
	if err := knownUnique(keyCapabilities, f.Capabilities, model.Capabilities()); err != nil {
		return err
	}
	return CheckCapabilities(f.Capabilities)
}

// validateRecords judges the waivers, the exceptions and the managed files.
func (f File) validateRecords() error {
	for i, w := range f.Waivers {
		key := fmt.Sprintf("vulnerability_waivers[%d]", i)
		if w.ID == "" {
			return invalid("%s has no id", key)
		}
		if err := dated(key, w.Until); err != nil {
			return err
		}
		if err := relPath(key+".record", w.Record); err != nil {
			return err
		}
	}
	for i, e := range f.Exceptions {
		if err := e.validate(fmt.Sprintf("exceptions[%d]", i)); err != nil {
			return err
		}
	}
	for i, m := range f.Managed {
		key := fmt.Sprintf("managed[%d]", i)
		if err := relPath(key+".path", m.Path); err != nil {
			return err
		}
		if !hexPattern.MatchString(m.SHA256) {
			return invalid("%s.sha256 %q is not 64 hex digits", key, m.SHA256)
		}
	}
	return nil
}

// validate judges one exception: a check id, a path, a record and a date.
func (e ExceptionRule) validate(key string) error {
	if !model.ValidID(e.Check) {
		return invalid("%s.check %q is not a check id", key, e.Check)
	}
	if err := relPath(key+".path", e.Path); err != nil {
		return err
	}
	if err := relPath(key+".record", e.Record); err != nil {
		return err
	}
	return dated(key, e.Until)
}

// dated refuses a missing or malformed expiry.
func dated(key, until string) error {
	if _, err := time.Parse(dateLayout, until); err != nil {
		return invalid("%s.until %q is not a date like 2026-12-31", key, until)
	}
	return nil
}

// relPath refuses a path that is not relative, clean and slash separated;
// a trailing slash marks a directory prefix and is allowed.
func relPath(key, p string) error {
	cleaned := path.Clean(p)
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, `\`) || cleaned == "." || cleaned == ".." ||
		strings.HasPrefix(cleaned, "../") || (cleaned != strings.TrimSuffix(p, "/") && cleaned+"/" != p) {
		return invalid("%s %q is not a clean relative path", key, p)
	}
	return nil
}

// knownUnique refuses an unknown or repeated entry of a list.
func knownUnique(key string, got, known []string) error {
	for i, name := range got {
		if !slices.Contains(known, name) {
			return invalid("%s names unknown %q", key, name)
		}
		if slices.Contains(got[:i], name) {
			return invalid("%s repeats %q", key, name)
		}
	}
	return nil
}
