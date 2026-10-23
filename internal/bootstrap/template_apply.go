package bootstrap

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/cristianargotti/golaw/internal/install"
)

// ApplyTemplates publishes the complete diff before the first atomic write.
func ApplyTemplates(root string, plan TemplatePlan, out io.Writer) error {
	resolved, err := templateRoot(root)
	if err != nil {
		return err
	}
	if err := verifyTemplates(resolved, plan); err != nil {
		return err
	}
	if err := WriteTemplateDiff(out, plan); err != nil {
		return err
	}
	if err := verifyTemplates(resolved, plan); err != nil {
		return err
	}
	for _, change := range plan.Changes {
		if err := writeTemplate(resolved, change); err != nil {
			return err
		}
	}
	return nil
}

// WriteTemplateDiff keeps previews and application output identical.
func WriteTemplateDiff(out io.Writer, plan TemplatePlan) error {
	if err := writeOwnership(out, plan); err != nil {
		return err
	}
	if err := writeExcluded(out, plan); err != nil {
		return err
	}
	if err := writeCollisions(out, plan); err != nil {
		return err
	}
	if err := writePromoted(out, plan); err != nil {
		return err
	}
	writes, proposals := "Gravações:", "Propostas sem gravação:"
	if plan.Language != "" && plan.Language != "pt-BR" {
		writes, proposals = "Writes:", "Proposals without writes:"
	}
	for _, group := range []struct {
		header  string
		changes []install.Change
	}{{writes, plan.Changes}, {proposals, plan.Proposed}} {
		if len(group.changes) == 0 {
			continue
		}
		if _, err := fmt.Fprintln(out, group.header); err != nil {
			return fmt.Errorf("bootstrap: write template header: %w", err)
		}
		if err := install.WriteDiff(out, group.changes); err != nil {
			return fmt.Errorf("bootstrap: write template diff: %w", err)
		}
	}
	return nil
}

func writeTemplate(root string, change install.Change) error {
	if err := verifyTemplate(root, change); err != nil {
		return err
	}
	path := filepath.Join(root, filepath.FromSlash(change.Path))
	if err := install.WriteAtomic(path, change.After); err != nil {
		return fmt.Errorf("bootstrap: write template %q: %w", change.Path, err)
	}
	return nil
}
