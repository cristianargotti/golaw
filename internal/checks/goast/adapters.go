package goast

import (
	"fmt"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

const (
	codeAdapterPackageSize = "adapter-package-size"
	adaptersDir            = "internal/adapters"
	adapterRootDepth       = 4
	defaultAdapterFiles    = 12
	optionMaxFiles         = "max_files"
)

// AdapterPackageSize is goast.adapter-package-size: the root of an adapter
// package, internal/adapters/<side>/<name>, keeps the shared plumbing only,
// so more than max_files non-test Go files in it is a finding. The check
// applies only when internal/adapters exists.
func AdapterPackageSize() model.Check {
	s := spec("adapter-package-size")
	s.Options = model.OptionSchema{{
		Name:    optionMaxFiles,
		Kind:    model.OptionInt,
		Tighten: model.TightenDown,
		Default: model.IntValue(defaultAdapterFiles),
	}}
	return check{spec: s, gate: requireDir(adaptersDir), judge: judgeAdapterPackageSize}
}

func judgeAdapterPackageSize(t *target, s *state) ([]model.Finding, error) {
	dir := t.dir()
	if !adapterRoot(dir) || !s.first("adapters:"+dir) {
		return nil, nil
	}
	limit, ok := s.in.Options.Int(optionMaxFiles)
	if !ok {
		limit = defaultAdapterFiles
	}
	count := 0
	for _, f := range s.siblings(dir) {
		if f.Class != model.ClassTest {
			count++
		}
	}
	if count <= limit {
		return nil, nil
	}
	message := fmt.Sprintf("%d non-test files in one adapter package, the limit is %d: the root keeps the shared plumbing and each context lives in its own package", count, limit)
	return []model.Finding{{Path: dir, Code: codeAdapterPackageSize, Message: message, Severity: model.SeverityDeny}}, nil
}

// adapterRoot reports a directory of the form internal/adapters/<side>/<name>.
func adapterRoot(dir string) bool {
	parts := strings.Split(dir, "/")
	return len(parts) == adapterRootDepth && strings.HasPrefix(dir, adaptersDir+"/")
}
