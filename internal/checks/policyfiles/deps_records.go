package policyfiles

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

type dependencyRecords struct{}

// DependencyRecords requires exact front-matter entries so prose cannot silently
// authorize a direct dependency that has not been reviewed.
func DependencyRecords() model.Check { return dependencyRecords{} }

func (dependencyRecords) Spec() model.Spec { return spec("dependency-records", ruleSupply, "go") }

func (dependencyRecords) Run(_ context.Context, in model.Input) (model.Result, error) {
	modules, err := dependencyModules(in)
	if err != nil {
		return dependencyAnswer(err), nil
	}
	recorded, err := decisionModules(in)
	if err != nil {
		return dependencyAnswer(err), nil
	}
	files := make([]string, 0, len(modules))
	var findings []model.Finding
	for _, unit := range modules {
		files = append(files, unit.file)
		for _, required := range unit.mod.direct {
			if !recorded[required.path] {
				message := fmt.Sprintf("record %s in the modules front matter of a decision under %s",
					required.path, in.Policy.Layout.Decisions)
				findings = append(findings, finding(unit.file, required.line, "dependency-record-missing", message))
			}
		}
	}
	return model.Result{
		Status: model.Measured(), Scope: "modules: " + strings.Join(files, ", ") + "; dependency mode: modules",
		Findings: findings,
	}, nil
}

func decisionModules(in model.Input) (map[string]bool, error) {
	directory := strings.TrimSuffix(in.Policy.Layout.Decisions, "/") + "/"
	files := model.Select(in.Snapshot, func(path string) bool {
		return strings.HasPrefix(path, directory) && strings.HasSuffix(strings.ToLower(path), recordExtension)
	})
	modules := make(map[string]bool)
	for _, file := range files {
		names, err := readRecord(in, file.Path)
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			modules[name] = true
		}
	}
	return modules, nil
}

func readRecord(in model.Input, path string) ([]string, error) {
	data, found, err := read(in, path)
	if err != nil {
		return nil, &dependencyReadError{file: path, err: err}
	}
	if !found {
		return nil, fmt.Errorf("%s: %w", path, model.ErrNotInSnapshot)
	}
	return recordModules(path, data)
}

func dependencyAnswer(err error) model.Result {
	if failed, ok := errors.AsType[*dependencyReadError](err); ok {
		return answer(failed.file, err)
	}
	return answer("go.mod", err)
}
