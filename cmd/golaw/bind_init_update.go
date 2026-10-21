package main

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

func (b binding) updateTemplates(ctx context.Context, root string, data []byte, pin string) (bootstrap.TemplatePlan, string, error) {
	config, err := policy.Decode(data)
	if err != nil {
		return bootstrap.TemplatePlan{}, "", initOperation("decode update settings", err)
	}
	owners, err := b.initOwnership(ctx, root)
	if err != nil {
		return bootstrap.TemplatePlan{}, "", err
	}
	config.Kit.Version = pin
	inputs, _, notice, err := b.initTemplateContext(ctx, root, config)
	if err != nil {
		return bootstrap.TemplatePlan{}, "", err
	}
	return renderUpdateTemplates(root, inputs, config.Managed, notice, owners)
}

func renderUpdateTemplates(root string, inputs assets.Context, recorded []policy.ManagedFile, notice string, owners []byte) (bootstrap.TemplatePlan, string, error) {
	var rendered []assets.Rendered
	if notice == "" {
		var err error
		rendered, err = assets.RenderAllContext(golaw.FS, inputs)
		if err != nil {
			return bootstrap.TemplatePlan{}, "", initOperation("render updated templates", err)
		}
	}
	allowed, ownership, err := filterInitOwnership(rendered, owners)
	if err != nil {
		return bootstrap.TemplatePlan{}, "", err
	}
	allowed, irregular, err := filterUpdateIrregular(root, allowed, recorded)
	if err != nil {
		return bootstrap.TemplatePlan{}, "", err
	}
	plan, err := bootstrap.PlanUpdate(root, allowed, recorded)
	if err != nil {
		return bootstrap.TemplatePlan{}, "", initOperation("plan updated templates", err)
	}
	plan.Language, plan.Ownership, plan.Irregular = inputs.Language, ownership, irregular
	lines := updateNotices(plan, rendered)
	if notice != "" {
		lines = append([]string{notice}, lines...)
	}
	return plan, strings.Join(lines, "\n"), nil
}

// filterUpdateIrregular inspects the recorded destinations and, once a kit
// record survives that filter, the kit and merged files a new version may
// add: the update plan reads each arrival, so a link or a file where a
// directory belongs is preserved and reported as init --apply reports it
// instead of stopping the update. Arrivals wait for a surviving record, so
// a repository that never adopted the templates keeps the M2 report and a
// record the filter preserves cannot enable new writes.
func filterUpdateIrregular(root string, rendered []assets.Rendered, recorded []policy.ManagedFile) ([]assets.Rendered, []string, error) {
	isRecorded := func(path string) bool {
		return slices.ContainsFunc(recorded, func(record policy.ManagedFile) bool { return record.Path == path })
	}
	kept, irregular, err := filterInitIrregular(root, rendered, isRecorded)
	if err != nil {
		return nil, nil, err
	}
	hasKitRecord := slices.ContainsFunc(kept, func(file assets.Rendered) bool {
		return file.Template.Ownership == assets.OwnedByKit && isRecorded(file.Path)
	})
	if !hasKitRecord {
		return kept, irregular, nil
	}
	paths := make([]string, 0, len(kept))
	for i := range kept {
		file := &kept[i]
		if !isRecorded(file.Path) && (file.Template.Ownership == assets.OwnedByKit || file.Template.Ownership == assets.OwnedMerged) {
			paths = append(paths, file.Path)
		}
	}
	kept, arrivals, err := filterInitIrregular(root, kept, func(path string) bool { return slices.Contains(paths, path) })
	if err != nil {
		return nil, nil, err
	}
	return kept, append(irregular, arrivals...), nil
}

func updateNotices(plan bootstrap.TemplatePlan, rendered []assets.Rendered) []string {
	lines := updateWriteNotices(plan)
	for _, recorded := range plan.Managed {
		if slices.ContainsFunc(rendered, func(file assets.Rendered) bool { return file.Path == recorded.Path }) {
			continue
		}
		message := "O manifesto atual não pode gerar %s com os dados atuais; hash registrado=%s."
		if plan.Language != model.DefaultLanguage {
			message = "The current manifest cannot render %s with the current inputs; recorded hash=%s."
		}
		lines = append(lines, fmt.Sprintf(message, recorded.Path, recorded.SHA256))
	}
	for _, edit := range plan.LocalEdits {
		lines = append(lines, updateLocalEdit(edit, plan.Language))
	}
	return lines
}

func updateWriteNotices(plan bootstrap.TemplatePlan) []string {
	message := "Atualização gerenciada: %s registrado=%s atual=%s proposto=%s."
	arrival := "Arquivo novo do kit: %s proposto=%s."
	if plan.Language != model.DefaultLanguage {
		message = "Managed update: %s recorded=%s current=%s proposed=%s."
		arrival = "New kit file: %s proposed=%s."
	}
	lines := make([]string, 0, len(plan.Changes))
	for _, change := range plan.Changes {
		if !change.Existed {
			lines = append(lines, fmt.Sprintf(arrival, change.Path, install.Digest(change.After)))
			continue
		}
		current := install.Digest(change.Before)
		lines = append(lines, fmt.Sprintf(message, change.Path, current, current, install.Digest(change.After)))
	}
	return lines
}

func updateLocalEdit(edit bootstrap.LocalEdit, language string) string {
	message, absent := "Edição local preservada: %s registrado=%s atual=%s proposto=%s.", "ausente"
	if language != model.DefaultLanguage {
		message, absent = "Local edit preserved: %s recorded=%s current=%s proposed=%s.", "absent"
	}
	current := edit.Current
	if current == "" {
		current = absent
	}
	return fmt.Sprintf(message, edit.Path, edit.Recorded, current, edit.Proposed)
}
