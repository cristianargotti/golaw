package paths

import (
	"context"
	"fmt"
	"regexp"

	"github.com/cristianargotti/golaw/internal/model"
)

// The directories where an image or a document may live, and the
// extensions that make a file one.
var (
	allowedBinary  = regexp.MustCompile(`^(?:public|manual)/|(?:^|/)(?:__)?fixtures(?:__)?/|(?:^|/)testdata/`)
	binaryDocument = regexp.MustCompile(`(?i)\.(?:png|jpe?g|gif|xlsx|docx|pdf)$`)
)

// forbidden is paths.forbidden.
type forbidden struct{}

// Forbidden is paths.forbidden: no image or document outside public/,
// manual/, fixtures/ or testdata/, and no file named like secret material
// anywhere in the tree.
func Forbidden() model.Check { return forbidden{} }

// Spec returns the check specification.
func (forbidden) Spec() model.Spec { return spec("paths.forbidden", model.ScopeFile) }

// Run judges the name of every target, binary files included.
func (forbidden) Run(_ context.Context, in model.Input) (model.Result, error) {
	var findings []model.Finding
	for _, f := range in.Targets {
		if code, message, ok := judge(f.Path); ok {
			findings = append(findings, deny(f.Path, 0, code, message))
		}
	}
	return model.Result{Status: model.Measured(), Scope: fmt.Sprintf("%d paths", len(in.Targets)), Findings: findings}, nil
}

// judge names the rule a path breaks: a binary document outside the
// allowed directories or a file named like secret material.
func judge(path string) (string, string, bool) {
	switch {
	case allowedBinary.MatchString(path):
		return "", "", false
	case binaryDocument.MatchString(path):
		return "binary-document", "image or document outside public/, manual/, fixtures/ or testdata/; binaries belong elsewhere", true
	case secretName(path):
		return "secret-file", "file named like secret material in the tree; secrets never live in the repository", true
	default:
		return "", "", false
	}
}
