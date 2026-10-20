package bootstrap

import (
	"bytes"
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/policy"
)

// LocalEdit retains the recorded baseline so ordinary init cannot adopt drift.
type LocalEdit struct {
	Path, Recorded, Current, Proposed string
}

// TemplatePlan keeps proposals distinct from writes and freezes their inputs.
type TemplatePlan struct {
	Changes    []install.Change
	Proposed   []install.Change
	Managed    []policy.ManagedFile
	LocalEdits []LocalEdit
	Language   string
	root       string
	inputs     []install.Change
}

// PlanTemplates preserves existing ownership before any output or mutation.
func PlanTemplates(root string, rendered []assets.Rendered) (TemplatePlan, error) {
	resolved, err := templateRoot(root)
	if err != nil {
		return TemplatePlan{}, err
	}
	current, err := Current(resolved)
	if err != nil {
		return TemplatePlan{}, err
	}
	managed, err := templateBaselines(current)
	if err != nil {
		return TemplatePlan{}, err
	}
	plan := TemplatePlan{
		root: resolved, Managed: managed,
		inputs: []install.Change{{Path: PolicyFile, Before: current, Existed: current != nil}},
	}
	for i := range rendered {
		if err := plan.addTemplate(rendered[i]); err != nil {
			return TemplatePlan{}, err
		}
	}
	slices.SortFunc(plan.Managed, func(a, b policy.ManagedFile) int { return cmp.Compare(a.Path, b.Path) })
	return plan, nil
}

func templateBaselines(current []byte) ([]policy.ManagedFile, error) {
	if current == nil {
		return nil, nil
	}
	config, err := policy.Decode(current)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: read managed baselines: %w", err)
	}
	return config.Managed, nil
}

func (plan *TemplatePlan) addTemplate(file assets.Rendered) error {
	if slices.ContainsFunc(plan.inputs, func(c install.Change) bool { return c.Path == file.Path }) {
		return fmt.Errorf("bootstrap: duplicate template destination %q: %w", file.Path, install.ErrPath)
	}
	before, existed, err := readTemplate(plan.root, file.Path)
	if err != nil {
		return err
	}
	change := install.Change{Path: file.Path, Before: before, After: bytes.Clone(file.Data), Existed: existed}
	plan.inputs = append(plan.inputs, change)
	if file.Template.Ownership == assets.OwnedMerged || file.Template.Ownership == assets.OwnedByDeveloper {
		plan.Managed = slices.DeleteFunc(plan.Managed, func(m policy.ManagedFile) bool { return m.Path == file.Path })
	}
	switch file.Template.Ownership {
	case assets.OwnedByKit:
		plan.addKitTemplate(change)
	case assets.OwnedMerged:
		plan.addMergedTemplate(change)
	case assets.OwnedByDeveloper:
		if !existed {
			plan.Changes = append(plan.Changes, change)
		}
	default:
		return fmt.Errorf("bootstrap: template %q ownership %q: %w", file.Path, file.Template.Ownership, assets.ErrManifest)
	}
	return nil
}

func (plan *TemplatePlan) addKitTemplate(change install.Change) {
	hash := install.Digest(change.After)
	index := slices.IndexFunc(plan.Managed, func(m policy.ManagedFile) bool { return m.Path == change.Path })
	if change.Existed && !bytes.Equal(change.Before, change.After) {
		plan.Proposed = append(plan.Proposed, change)
		if index >= 0 && plan.Managed[index].SHA256 != install.Digest(change.Before) {
			plan.LocalEdits = append(plan.LocalEdits, LocalEdit{
				Path: change.Path, Recorded: plan.Managed[index].SHA256,
				Current: install.Digest(change.Before), Proposed: hash,
			})
		}
		return
	}
	if !change.Existed {
		plan.Changes = append(plan.Changes, change)
	}
	record := policy.ManagedFile{Path: change.Path, SHA256: hash}
	if index >= 0 {
		plan.Managed[index] = record
		return
	}
	plan.Managed = append(plan.Managed, record)
}

func (plan *TemplatePlan) addMergedTemplate(change install.Change) {
	if !change.Existed {
		plan.Changes = append(plan.Changes, change)
		return
	}
	change.After = appendTemplateLines(change.Before, change.After)
	if !bytes.Equal(change.Before, change.After) {
		plan.Proposed = append(plan.Proposed, change)
	}
}

func appendTemplateLines(before, rendered []byte) []byte {
	current, required := map[string]int{}, map[string]int{}
	for line := range strings.SplitSeq(string(before), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) != "" {
			current[line]++
		}
	}
	after := bytes.Clone(before)
	newline := policyNewline(before)
	for line := range strings.SplitSeq(string(rendered), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		required[line]++
		if current[line] >= required[line] {
			continue
		}
		if len(after) != 0 && after[len(after)-1] != '\n' {
			after = append(after, newline...)
		}
		after = append(after, line...)
		after = append(after, newline...)
		current[line]++
	}
	return after
}
