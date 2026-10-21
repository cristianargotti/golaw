package assets

import (
	"fmt"
	"io/fs"
	"strings"
)

// ServicePrefix separates the example service from templates init applies.
const ServicePrefix = "templates/examples/service/"

// RenderService keeps the fixture out of templates/manifest.yml because it
// is built for proof and never rendered into an adopter.
func RenderService(source fs.FS, values Values) ([]Rendered, error) {
	files, err := Load(source)
	if err != nil {
		return nil, err
	}
	var rendered []Rendered
	for _, file := range files {
		if !strings.HasPrefix(file.Path, ServicePrefix) {
			continue
		}
		entry, err := serviceTemplate(file.Path)
		if err != nil {
			return nil, err
		}
		result, err := Render(source, entry, values)
		if err != nil {
			return nil, err
		}
		rendered = append(rendered, result)
	}
	if len(rendered) == 0 {
		return nil, fmt.Errorf("%w: no service fixture under %s", ErrManifest, ServicePrefix)
	}
	return rendered, nil
}

func serviceTemplate(path string) (Template, error) {
	destination, ok := strings.CutSuffix(path, ".tmpl")
	if !ok {
		return Template{}, fmt.Errorf("%w: service fixture file %s is not a template", ErrManifest, path)
	}
	return Template{
		Source: path, Destination: strings.TrimPrefix(destination, ServicePrefix),
		Ownership: OwnedByDeveloper,
	}, nil
}
