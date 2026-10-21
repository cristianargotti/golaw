package bootstrap

import (
	"bytes"
	"cmp"
	"fmt"
	"slices"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// PlanUpdate limits rewrites to recorded baselines so local ownership
// survives version movement, and plans the kit files a newer version adds.
func PlanUpdate(root string, rendered []assets.Rendered, recorded []policy.ManagedFile) (TemplatePlan, error) {
	plan, err := PlanTemplates(root, nil)
	if err != nil {
		return TemplatePlan{}, err
	}
	records, err := updateRecords(recorded)
	if err != nil {
		return TemplatePlan{}, err
	}
	files, err := updateTemplates(rendered)
	if err != nil {
		return TemplatePlan{}, err
	}
	plan.Managed = nil
	for _, record := range records {
		file, known := files[record.Path]
		if !known {
			plan.Managed = append(plan.Managed, record)
			continue
		}
		if err := plan.addUpdate(file, record); err != nil {
			return TemplatePlan{}, err
		}
	}
	if err := plan.addArrivals(files, records); err != nil {
		return TemplatePlan{}, err
	}
	slices.SortFunc(plan.Managed, func(a, b policy.ManagedFile) int { return cmp.Compare(a.Path, b.Path) })
	return plan, nil
}

func updateRecords(recorded []policy.ManagedFile) ([]policy.ManagedFile, error) {
	records := make([]policy.ManagedFile, 0, len(recorded))
	hashes := make(map[string]string, len(recorded))
	for _, record := range recorded {
		if previous, exists := hashes[record.Path]; exists {
			if previous != record.SHA256 {
				return nil, fmt.Errorf("bootstrap: conflicting managed baselines for %q: %w", record.Path, model.ErrPolicy)
			}
			continue
		}
		hashes[record.Path] = record.SHA256
		records = append(records, record)
	}
	return records, nil
}

func updateTemplates(rendered []assets.Rendered) (map[string]assets.Rendered, error) {
	files := make(map[string]assets.Rendered, len(rendered))
	for i := range rendered {
		file := &rendered[i]
		if _, exists := files[file.Path]; exists {
			return nil, fmt.Errorf("bootstrap: duplicate template destination %q: %w", file.Path, install.ErrPath)
		}
		files[file.Path] = *file
	}
	return files, nil
}

func (plan *TemplatePlan) addUpdate(file assets.Rendered, record policy.ManagedFile) error {
	if record.Path == PolicyFile {
		return fmt.Errorf("bootstrap: template destination %q: %w", record.Path, install.ErrPath)
	}
	before, existed, err := readTemplate(plan.root, record.Path)
	if err != nil {
		return err
	}
	change := install.Change{Path: record.Path, Before: before, After: bytes.Clone(file.Data), Existed: existed}
	plan.inputs = append(plan.inputs, change)
	switch file.Template.Ownership {
	case assets.OwnedByKit:
		plan.updateKit(change, record)
	case assets.OwnedMerged:
		if existed {
			plan.addMergedTemplate(change)
		}
	case assets.OwnedByDeveloper:
	default:
		return fmt.Errorf("bootstrap: template %q ownership %q: %w", file.Path, file.Template.Ownership, assets.ErrManifest)
	}
	return nil
}

func (plan *TemplatePlan) updateKit(change install.Change, record policy.ManagedFile) {
	proposed := install.Digest(change.After)
	current := ""
	if change.Existed {
		current = install.Digest(change.Before)
	}
	if change.Existed && (current == record.SHA256 || current == proposed) {
		if current != proposed {
			plan.Changes = append(plan.Changes, change)
		}
		record.SHA256 = proposed
	} else {
		plan.LocalEdits = append(plan.LocalEdits, LocalEdit{
			Path: record.Path, Recorded: record.SHA256, Current: current, Proposed: proposed,
		})
	}
	plan.Managed = append(plan.Managed, record)
}
