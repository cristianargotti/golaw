package main

import (
	"bytes"
	"fmt"
	"slices"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/model"
)

func filterInitOwnership(rendered []assets.Rendered, codeowners []byte) ([]assets.Rendered, bootstrap.Ownership, error) {
	paths := make([]string, 1, len(rendered)+1)
	paths[0] = bootstrap.PolicyFile
	for i := range rendered {
		paths = append(paths, rendered[i].Path)
	}
	ownership, err := bootstrap.AllowedPaths(paths, codeowners)
	if err != nil {
		return nil, bootstrap.Ownership{}, initOperation("read destination ownership", err)
	}
	allowed := make([]assets.Rendered, 0, len(rendered))
	for i := range rendered {
		if slices.Contains(ownership.Allowed, rendered[i].Path) {
			allowed = append(allowed, rendered[i])
		}
	}
	return allowed, ownership, nil
}

// checkInitOwnership waits for final bytes so enrollment alone cannot be
// refused. golaw cannot tell who runs it, so the remedy names what the
// owners commit by hand: the bytes the preview prints.
func checkInitOwnership(plan initPlan) error {
	if !slices.Contains(plan.templates.Ownership.Protected, bootstrap.PolicyFile) ||
		plan.current != nil && bytes.Equal(plan.current, plan.data) {
		return nil
	}
	preview, apply := "golaw init", "golaw init --apply"
	if plan.update {
		preview, apply = "golaw init --update", "golaw init --update --apply"
	}
	return fmt.Errorf("%w: CODEOWNERS protects golaw.yml; one of its owners must commit golaw.yml and the files %s previews, then %s enrolls it",
		model.ErrUsage, preview, apply)
}
