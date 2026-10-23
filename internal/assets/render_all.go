package assets

import (
	"cmp"
	"fmt"
	"io/fs"
	"slices"
)

// RenderAllContext detects expanded collisions before any output is installed.
func RenderAllContext(source fs.FS, context Context) ([]Rendered, error) {
	entries, err := Templates(source)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRender, err)
	}
	files := make([]Rendered, 0, len(entries))
	for _, entry := range entries {
		if !selectedTemplate(entry, context.Capabilities) {
			continue
		}
		file, err := RenderContext(source, entry, context)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	slices.SortFunc(files, func(a, b Rendered) int { return cmp.Compare(a.Path, b.Path) })
	if err := validateRenderedPaths(files); err != nil {
		return nil, err
	}
	return files, nil
}

func selectedTemplate(entry Template, capabilities []string) bool {
	for _, capability := range entry.When {
		if !slices.Contains(capabilities, capability) {
			return false
		}
	}
	return true
}

func validateRenderedPaths(files []Rendered) error {
	for i := 1; i < len(files); i++ {
		if files[i-1].Path == files[i].Path {
			return fmt.Errorf("%w: sources %q and %q share destination %q", ErrRender,
				files[i-1].Template.Source, files[i].Template.Source, files[i].Path)
		}
	}
	return nil
}
