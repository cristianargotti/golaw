package paths

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// ignoreFile is the name of a git ignore file at any depth.
const ignoreFile = ".gitignore"

// ignoredTree matches an ignore rule whose last element is a generated tree,
// with or without a trailing slash or star, at the root, under a path or
// under a double star.
var ignoredTree = regexp.MustCompile(`(?:^|/)(?:gen|sqlcgen|test/mocks|api/generated)(?:/\**|/)?$`)

type generatedTracked struct{}

// GeneratedTracked is paths.generated-tracked: no ignore file hides gen/,
// sqlcgen/, test/mocks or api/generated, because generated code is versioned
// and proven fresh by the pipeline.
func GeneratedTracked() model.Check { return generatedTracked{} }

func (generatedTracked) Spec() model.Spec { return spec("paths.generated-tracked", model.ScopeFile) }

// Run reads every ignore file among the targets, an unreadable one
// included: its failed read answers Unmeasured instead of clean, and one
// over the read bound is left unread and named.
func (generatedTracked) Run(_ context.Context, in model.Input) (model.Result, error) {
	var findings []model.Finding
	var unread []string
	files := 0
	for _, f := range in.Targets {
		readable := f.Class.IsText() || f.Class == model.ClassUnreadable
		if !readable || (f.Path != ignoreFile && !strings.HasSuffix(f.Path, "/"+ignoreFile)) {
			continue
		}
		content, err := in.Snapshot.Read(f.Path)
		if errors.Is(err, model.ErrTooLarge) {
			unread = append(unread, f.Path)
			continue
		}
		if err != nil {
			return unreadable(f.Path, err), nil
		}
		files++
		findings = append(findings, ignoredTrees(f.Path, string(content))...)
	}
	return model.Result{Status: model.Measured(), Scope: fmt.Sprintf("%d ignore files", files), Findings: findings, Unread: unread}, nil
}

// ignoredTrees reports every rule of an ignore file that hides a generated
// tree; comments and negations are skipped. A rule is read from the folder
// of its ignore file, as git reads it, so mocks/ in test/.gitignore hides
// test/mocks.
func ignoredTrees(path, content string) []model.Finding {
	var out []model.Finding
	folder := strings.TrimSuffix(path, ignoreFile)
	for i, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		if ignoredTree.MatchString(folder + strings.TrimPrefix(line, "/")) {
			message := fmt.Sprintf("ignore rule %q hides a generated tree; generated code is versioned and proven fresh", line)
			out = append(out, deny(path, i+1, "generated-ignored", message))
		}
	}
	return out
}

// unreadable is the result of a check whose target could not be read.
func unreadable(path string, err error) model.Result {
	return model.Result{Status: model.Unmeasured(fmt.Errorf("read %s: %w", path, err).Error())}
}
