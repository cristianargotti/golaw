package bootstrap

import (
	"cmp"
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/policy"
)

// Accept requires an exact proposal so consent cannot select an unrelated file.
func (plan *TemplatePlan) Accept(paths []string) error {
	accepted := make(map[string]struct{}, len(paths))
	for _, name := range paths {
		destination := path.Clean(filepath.ToSlash(name))
		if _, found := accepted[destination]; found {
			continue
		}
		if err := plan.accept(destination); err != nil {
			return err
		}
		accepted[destination] = struct{}{}
	}
	return nil
}

func (plan *TemplatePlan) accept(destination string) error {
	if destination == "go.mod" {
		return fmt.Errorf("%w: go.mod stays the person's file: apply its proposal with go get or go mod edit", ErrAccept)
	}
	index := slices.IndexFunc(plan.Proposed, func(c install.Change) bool { return c.Path == destination })
	if index < 0 {
		return fmt.Errorf("%w: %s; the proposals of this run are: %s", ErrAccept, destination, plan.proposalNames())
	}
	if destination == "Makefile" && len(plan.Collisions) > 0 {
		var targets []string
		for _, collision := range plan.Collisions {
			targets = append(targets, fmt.Sprintf("%s (Makefile:%d)", collision.Target, collision.Line))
		}
		return fmt.Errorf("%w: Makefile while %s also define golaw.mk targets", ErrAccept, strings.Join(targets, ", "))
	}
	change := plan.Proposed[index]
	plan.Proposed = slices.Delete(plan.Proposed, index, index+1)
	plan.Changes = append(plan.Changes, change)
	slices.SortFunc(plan.Changes, func(a, b install.Change) int { return cmp.Compare(a.Path, b.Path) })
	if plan.owners[destination] == assets.OwnedByKit {
		plan.acceptKit(change)
	}
	return nil
}

func (plan *TemplatePlan) proposalNames() string {
	names := make([]string, 0, len(plan.Proposed))
	for _, change := range plan.Proposed {
		names = append(names, change.Path)
	}
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

func (plan *TemplatePlan) acceptKit(change install.Change) {
	record := policy.ManagedFile{Path: change.Path, SHA256: install.Digest(change.After)}
	index := slices.IndexFunc(plan.Managed, func(m policy.ManagedFile) bool { return m.Path == change.Path })
	if index < 0 {
		plan.Managed = append(plan.Managed, record)
	} else {
		plan.Managed[index] = record
	}
	slices.SortFunc(plan.Managed, func(a, b policy.ManagedFile) int { return cmp.Compare(a.Path, b.Path) })
	plan.LocalEdits = slices.DeleteFunc(plan.LocalEdits, func(edit LocalEdit) bool { return edit.Path == change.Path })
}
