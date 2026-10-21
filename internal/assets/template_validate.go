package assets

import (
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

func validateTemplates(source fs.FS, entries []Template) error {
	previous := ""
	for _, entry := range entries {
		if entry.Destination <= previous {
			return fmt.Errorf("%w: destinations must be unique and sorted: %q", ErrManifest, entry.Destination)
		}
		if err := validateTemplate(source, entry); err != nil {
			return err
		}
		previous = entry.Destination
	}
	return nil
}

func validateTemplate(source fs.FS, entry Template) error {
	if !templatePath(entry.Source) || !strings.HasPrefix(entry.Source, "templates/") ||
		!strings.HasSuffix(entry.Source, ".tmpl") {
		return fmt.Errorf("%w: source must be a .tmpl under templates/: %q", ErrManifest, entry.Source)
	}
	destination := strings.ReplaceAll(entry.Destination, "{{.Decisions}}", "decisions")
	if !templatePath(destination) || strings.Contains(destination, "{{") || strings.Contains(destination, "}}") {
		return fmt.Errorf("%w: invalid destination: %q", ErrManifest, entry.Destination)
	}
	switch entry.Ownership {
	case OwnedByKit, OwnedMerged, OwnedByDeveloper:
	default:
		return fmt.Errorf("%w: unknown ownership: %q", ErrManifest, entry.Ownership)
	}
	for i, capability := range entry.When {
		if !slices.Contains(model.Capabilities(), capability) {
			return fmt.Errorf("%w: unknown capability: %q", ErrManifest, capability)
		}
		if slices.Contains(entry.When[:i], capability) {
			return fmt.Errorf("%w: capability %q repeats in the condition of %s", ErrManifest, capability, entry.Source)
		}
	}
	if _, err := fs.ReadFile(source, entry.Source); err != nil {
		return fmt.Errorf("%w: read template %s: %w", ErrManifest, entry.Source, err)
	}
	return nil
}

func templatePath(name string) bool {
	return name != "." && fs.ValidPath(name) && !strings.ContainsAny(name, "\\:\x00")
}

func templateValue(value *templateFields) (Template, error) {
	if value == nil {
		return Template{}, fmt.Errorf("%w: template entry must be a mapping", ErrManifest)
	}
	when := make([]string, len(value.When))
	for i, capability := range value.When {
		if capability == nil {
			return Template{}, fmt.Errorf("%w: capability must be a known name, not null", ErrManifest)
		}
		when[i] = *capability
	}
	return Template{
		Source: value.Source, Destination: value.Destination,
		Ownership: value.Ownership, When: when,
	}, nil
}
