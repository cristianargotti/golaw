package paths

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

type profileForbidden struct{}

// ProfileForbidden is paths.profile-forbidden: a path the enrolled profile
// forbids, judged by its name in every mode a file check runs in, so an
// agent's write is denied before the file exists (section 5.4, D5).
func ProfileForbidden() model.Check { return profileForbidden{} }

func (profileForbidden) Spec() model.Spec { return spec("paths.profile-forbidden", model.ScopeFile) }

// Run judges the name of every target outside a testdata directory.
func (profileForbidden) Run(_ context.Context, in model.Input) (model.Result, error) {
	switch {
	case in.Policy.Profile == "":
		return model.Result{Status: model.NotApplicable("no profile is enrolled; the profile turns this rule on")}, nil
	case !in.Policy.ProfileData.Section(model.SectionPaths):
		return model.Result{Status: model.NotApplicable("the profile carries no paths")}, nil
	}
	var findings []model.Finding
	for _, f := range in.Targets {
		if f.Class == model.ClassFixture {
			continue
		}
		if glob, ok := forbiddenBy(in.Policy.ProfileData.Paths, f.Path); ok {
			message := fmt.Sprintf("%s is a path the profile forbids (%s): remove it or keep it out of the repository", f.Path, glob)
			findings = append(findings, deny(f.Path, 0, "profile-forbidden-path", message))
		}
	}
	return model.Result{Status: model.Measured(), Scope: fmt.Sprintf("%d paths", len(in.Targets)), Findings: findings}, nil
}

// forbiddenBy answers the glob of the profile a slash path matches: a glob
// that ends in a slash forbids every path under a directory it matches,
// any other one matches the whole path, both in the path.Match grammar and
// without case, so a name a case-insensitive filesystem aliases to a
// forbidden one is that name (A14).
func forbiddenBy(globs []string, p string) (string, bool) {
	name := strings.ToLower(p)
	for _, glob := range globs {
		dir, isDir := strings.CutSuffix(strings.ToLower(glob), "/")
		if isDir && underGlob(dir, name) || !isDir && matches(dir, name) {
			return glob, true
		}
	}
	return "", false
}

// underGlob reports whether a directory of the path matches the glob.
func underGlob(dir, p string) bool {
	for i := range len(p) {
		if p[i] == '/' && matches(dir, p[:i]) {
			return true
		}
	}
	return false
}

// matches is path.Match without its error: the loader validated every glob.
func matches(glob, p string) bool {
	matched, err := path.Match(glob, p)
	return err == nil && matched
}
