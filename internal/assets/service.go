package assets

import (
	"fmt"
	"io/fs"
	"path"
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

// sumFile is the one fixture file carried under its own name: a go.sum
// lists the requirements and never names the module, so it holds no
// placeholder, and the tools that know a go.sum by its name, the secret
// scan's own allowlist among them, must see it as one.
const sumFile = "go.sum"

func serviceTemplate(source string) (Template, error) {
	destination, ok := strings.CutSuffix(source, ".tmpl")
	if !ok && path.Base(source) != sumFile {
		return Template{}, fmt.Errorf("%w: service fixture file %s is not a template", ErrManifest, source)
	}
	return Template{
		Source: source, Destination: strings.TrimPrefix(destination, ServicePrefix),
		Ownership: OwnedByDeveloper,
	}, nil
}
