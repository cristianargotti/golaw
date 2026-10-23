package profile

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// The field names the causes cite more than once, and the one layout
// value the model names no constant for.
const (
	fieldInstalls = "dockerfile.installs"
	mainCmd       = "cmd"
)

// packagePattern is a package name an install allowlist may carry: what
// apk and apt accept, without a version, which pinned_installs judges.
var packagePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9+._-]*$`)

// Validate judges decoded data (D4): every term, target and stage lower
// case kebab case, a stage named once, every path and template prefix
// relative, clean and in slash form, the registry pattern an RE2 that
// compiles, every assertion with a path, a key, a known kind, a value of
// that kind, a known missing rule and a known files rule, a layout main
// of root or cmd with a relative decisions directory, and no empty image
// or registry section. Decode refuses the reserved key branch before.
func Validate(d model.ProfileData) error {
	if err := validate(d); err != nil {
		return bundlef("%v", err)
	}
	return nil
}

// validate is Validate with the cause alone, so Read can name the profile.
func validate(d model.ProfileData) error {
	for _, check := range []func(model.ProfileData) error{
		validateNames, validatePaths, validateLayout, validateDockerfile, validateRegistry, validateAssertions,
	} {
		if err := check(d); err != nil {
			return err
		}
	}
	return nil
}

// validateNames judges the terms and the targets.
func validateNames(d model.ProfileData) error {
	for _, term := range d.Terms {
		if !model.ValidCode(term) {
			return causef("%s: %q is not lower case kebab case", model.SectionTerms, term)
		}
	}
	for _, target := range d.Targets {
		if !model.ValidCode(target) {
			return causef("%s: %q is not lower case kebab case", model.SectionTargets, target)
		}
	}
	return nil
}

// validatePaths judges the forbidden paths and the template prefixes.
func validatePaths(d model.ProfileData) error {
	for _, p := range d.Paths {
		if err := relPath(model.SectionPaths, p); err != nil {
			return err
		}
		if !validGlob(p) {
			return causef("%s: %q is not a valid glob", model.SectionPaths, p)
		}
	}
	for _, p := range d.Templates.Exclude {
		if err := relPath("templates.exclude", p); err != nil {
			return err
		}
	}
	return nil
}

// relPath refuses a path that is not relative, clean and slash separated;
// a trailing slash marks a directory prefix and is allowed.
func relPath(field, p string) error {
	cleaned := path.Clean(p)
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, `\`) || cleaned == "." || cleaned == ".." ||
		strings.HasPrefix(cleaned, "../") || (cleaned != strings.TrimSuffix(p, "/") && cleaned+"/" != p) {
		return causef("%s: %q is not a clean relative path", field, p)
	}
	return nil
}

// validGlob reports whether a pattern compiles in the path.Match grammar.
func validGlob(p string) bool {
	_, err := path.Match(p, "")
	return err == nil
}

// validateLayout judges the layout defaults init takes under the profile.
func validateLayout(d model.ProfileData) error {
	if d.Layout == nil {
		return nil
	}
	if d.Layout.Main != model.DefaultMain && d.Layout.Main != mainCmd {
		return causef("layout.main: %q is not root or cmd", d.Layout.Main)
	}
	return relPath("layout.decisions", d.Layout.Decisions)
}

// validateDockerfile judges the image rules.
func validateDockerfile(d model.ProfileData) error {
	f := d.Dockerfile
	if f == nil {
		return nil
	}
	if len(f.Stages) == 0 && f.FinalImage == "" && len(f.Installs) == 0 && !f.PinnedInstalls && len(f.IgnoreComments) == 0 {
		return causef("%s: the section is empty", model.SectionDockerfile)
	}
	for i, stage := range f.Stages {
		switch {
		case !model.ValidCode(stage):
			return causef("dockerfile.stages: %q is not lower case kebab case", stage)
		case slices.Contains(f.Stages[:i], stage):
			return causef("dockerfile.stages: %q is named twice", stage)
		}
	}
	if strings.ContainsAny(f.FinalImage, " \t") {
		return causef("dockerfile.final_image: %q is not one image reference", f.FinalImage)
	}
	for _, comment := range f.IgnoreComments {
		if !strings.HasPrefix(comment, "#") {
			return causef("dockerfile.ignore_comments: %q does not start with #", comment)
		}
	}
	return validateInstalls(f.Stages, f.Installs)
}

// validateInstalls judges the allowlists: a stage among the named stages
// when the profile orders them, and package names without a version.
func validateInstalls(stages []string, installs []model.InstallAllowlist) error {
	for _, allow := range installs {
		switch {
		case !model.ValidCode(allow.Stage):
			return causef("%s: stage %q is not lower case kebab case", fieldInstalls, allow.Stage)
		case len(stages) > 0 && !slices.Contains(stages, allow.Stage):
			return causef("%s: stage %q is not among dockerfile.stages", fieldInstalls, allow.Stage)
		case len(allow.Packages) == 0:
			return causef("%s: stage %s names no package", fieldInstalls, allow.Stage)
		}
		for _, pkg := range allow.Packages {
			if !packagePattern.MatchString(pkg) {
				return causef("%s: %q is not a package name", fieldInstalls, pkg)
			}
		}
	}
	return nil
}

// validateRegistry judges the image reference pattern.
func validateRegistry(d model.ProfileData) error {
	if d.Registry == nil {
		return nil
	}
	if d.Registry.Pattern == "" {
		return causef("registry.pattern: the pattern is empty")
	}
	if _, err := regexp.Compile(d.Registry.Pattern); err != nil {
		return causef("registry.pattern: %v", err)
	}
	return nil
}

// validateAssertions judges every typed assertion by its index.
func validateAssertions(d model.ProfileData) error {
	for i, a := range d.Assertions {
		if err := validateAssertion(fmt.Sprintf("assertions[%d]", i), a); err != nil {
			return err
		}
	}
	return nil
}

// validateAssertion judges one assertion: a clean relative glob, a key
// path with no empty segment, a known kind and explicit rules.
func validateAssertion(field string, a model.Assertion) error {
	kinds := []string{model.AssertBool, model.AssertInt, model.AssertString}
	switch {
	case relPath(field, a.Path) != nil || !validGlob(a.Path):
		return causef("%s.path: %q is not a clean relative glob", field, a.Path)
	case len(a.Key) == 0 || slices.Contains(a.Key, ""):
		return causef("%s.key: the key path needs one segment or more, none empty", field)
	case !slices.Contains(kinds, a.Kind):
		return causef("%s.kind: %q is not bool, int or string", field, a.Kind)
	case a.Missing != model.MissingFinding && a.Missing != model.MissingIgnore:
		return causef("%s.missing: %q is not finding or ignore", field, a.Missing)
	case a.Files != model.FilesRequired && a.Files != model.FilesOptional:
		return causef("%s.files: %q is not required or optional", field, a.Files)
	}
	return validateEquals(field, a)
}

// validateEquals judges the expected scalar against the kind.
func validateEquals(field string, a model.Assertion) error {
	if a.Equals == nil {
		return nil
	}
	switch a.Kind {
	case model.AssertBool:
		if *a.Equals != "true" && *a.Equals != "false" {
			return causef("%s.equals: %q is not true or false", field, *a.Equals)
		}
	case model.AssertInt:
		if _, err := strconv.ParseInt(*a.Equals, 10, 64); err != nil {
			return causef("%s.equals: %q is not an integer", field, *a.Equals)
		}
	}
	return nil
}
