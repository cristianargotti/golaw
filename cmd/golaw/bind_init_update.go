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
	config.Kit.Version = pin
	inputs, notice, err := b.initTemplateContext(ctx, root, config)
	if err != nil {
		return bootstrap.TemplatePlan{}, "", err
	}
	return renderUpdateTemplates(root, inputs, config.Managed, notice)
}

func renderUpdateTemplates(root string, inputs assets.Context, recorded []policy.ManagedFile, notice string) (bootstrap.TemplatePlan, string, error) {
	var rendered []assets.Rendered
	if notice == "" {
		var err error
		rendered, err = assets.RenderAllContext(golaw.FS, inputs)
		if err != nil {
			return bootstrap.TemplatePlan{}, "", initOperation("render updated templates", err)
		}
	}
	plan, err := bootstrap.PlanUpdate(root, rendered, recorded)
	if err != nil {
		return bootstrap.TemplatePlan{}, "", initOperation("plan updated templates", err)
	}
	plan.Language = inputs.Language
	lines := updateNotices(plan, rendered)
	if notice != "" {
		lines = append([]string{notice}, lines...)
	}
	return plan, strings.Join(lines, "\n"), nil
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
	if plan.Language != model.DefaultLanguage {
		message = "Managed update: %s recorded=%s current=%s proposed=%s."
	}
	lines := make([]string, 0, len(plan.Changes))
	for _, change := range plan.Changes {
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
