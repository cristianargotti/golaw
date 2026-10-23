package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
	scanreport "github.com/cristianargotti/golaw/internal/report"
)

// The two rows of A14 differ only in where the replace points: inside the
// repository the check follows it, outside it the check states the limit.
const (
	caseInternalReplace = "internal-replace"
	caseExternalReplace = "external-replace"
	dependencyCheck     = "policy.dependency-records"
	dependencyModule    = "example.test/library"
	dependencyCommit    = "build: require the library through a replace"
	externalExit        = 2
)

func (s *suite) dependencyRows(ctx context.Context) []row {
	return []row{
		measured(categoryDependency, caseInternalReplace, s.replaceRow(ctx, caseInternalReplace, "./library")),
		measured(categoryDependency, caseExternalReplace, s.replaceRow(ctx, caseExternalReplace, "../external-library")),
	}
}

func (s *suite) replaceRow(ctx context.Context, name, target string) error {
	repo, err := s.replaceRepository(ctx, name, target)
	if err != nil {
		return err
	}
	got := s.execute(ctx, command{program: s.binary, args: []string{commandScan, m2JSONFlag, "."}, dir: repo, env: s.env})
	return judgeReplace(name, target, got)
}

// replaceRepository writes the module and its library before init, so the
// baseline init seeds records the require and the only verdict left to the
// scan is whether it can follow the replace.
func (s *suite) replaceRepository(ctx context.Context, name, target string) (string, error) {
	repo, err := s.repository(ctx, name, false)
	if err != nil {
		return "", err
	}
	library := filepath.Join(repo, filepath.FromSlash(target))
	files := map[string]string{
		filepath.Join(repo, moduleFile): fmt.Sprintf("module example.invalid/%s\n\ngo 1.27\n\nrequire %s v1.0.0\n\nreplace %s => %s\n",
			name, dependencyModule, dependencyModule, target),
		filepath.Join(library, moduleFile):   "module " + dependencyModule + "\n\ngo 1.27\n",
		filepath.Join(library, "library.go"): "// Package library is the module the replace directive names.\npackage library\n",
	}
	for path, body := range files {
		if err := writeFile(path, []byte(body), fileMode); err != nil {
			return "", err
		}
	}
	if err := s.gitSteps(ctx, repo, []string{commandInit, m2Quiet}); err != nil {
		return "", err
	}
	adopt := command{program: s.binary, args: []string{commandInit, flagApply}, dir: repo, env: s.env}
	if err := success(s.execute(ctx, adopt)); err != nil {
		return "", err
	}
	return repo, s.gitSteps(ctx, repo, []string{gitAdd, "-A"}, commitArgs(dependencyCommit))
}

// judgeReplace holds the external row to a reason that names the target,
// because that reason is all a person has to find the directive to change.
func judgeReplace(name, target string, got result) error {
	var doc scanreport.Document
	if err := json.Unmarshal([]byte(got.stdout), &doc); err != nil {
		return fmt.Errorf("decode the scan of %s, exit %d %s: %w",
			name, got.code, strings.TrimSpace(got.stderr), errors.Join(err, got.err))
	}
	index := slices.IndexFunc(doc.Checks, func(item scanreport.Status) bool { return item.ID == dependencyCheck })
	if index < 0 {
		return fmt.Errorf("the scan of %s lacks %s: %w", name, dependencyCheck, ErrAcceptance)
	}
	check := doc.Checks[index]
	code, status, named := 0, model.StatusMeasured.String(), true
	if name == caseExternalReplace {
		code, status, named = externalExit, model.StatusUnmeasured.String(), strings.Contains(check.Reason, target)
	}
	if got.code != code || check.Status != status || !named {
		return fmt.Errorf("%s: exit %d, %s %s, reason %q: %w",
			name, got.code, dependencyCheck, check.Status, check.Reason, ErrAcceptance)
	}
	return nil
}
