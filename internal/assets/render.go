package assets

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"text/template"
)

// Values keeps adopter inputs explicit so rendering never probes a machine.
type Values struct {
	ModulePath   string
	GoVersion    string
	KitVersion   string
	Language     string
	Capabilities []string
	Decisions    string
	Main         string
	Tools        map[string]string
}

// Rendered binds ownership and destination to the exact bytes being applied.
type Rendered struct {
	Template Template
	Path     string
	Data     []byte
	SHA256   string
}

// Render keeps the fixed inputs usable without selecting a CI backend.
func Render(source fs.FS, t Template, v Values) (Rendered, error) {
	return RenderContext(source, t, Context{Values: v, CIBackend: "none"})
}

// RenderContext names both template paths when rendering cannot complete.
func RenderContext(source fs.FS, t Template, c Context) (Rendered, error) {
	result, err := renderContext(source, t, c)
	if err != nil {
		return Rendered{}, fmt.Errorf("%w: source %q destination %q: %w", ErrRender, t.Source, t.Destination, err)
	}
	return result, nil
}

func renderContext(source fs.FS, t Template, c Context) (Rendered, error) {
	destination, err := executeTemplate(t.Destination, c)
	if err != nil {
		return Rendered{}, err
	}
	path := string(destination)
	if !templatePath(path) {
		return Rendered{}, fmt.Errorf("%w: invalid expanded destination %q", ErrRender, path)
	}
	body, err := fs.ReadFile(source, t.Source)
	if err != nil {
		return Rendered{}, fmt.Errorf("read template: %w", err)
	}
	data, err := executeTemplate(string(body), c)
	if err != nil {
		return Rendered{}, err
	}
	if err := parseRendered(path, data); err != nil {
		return Rendered{}, err
	}
	return Rendered{Template: t, Path: path, Data: data, SHA256: fmt.Sprintf("%x", sha256.Sum256(data))}, nil
}

func executeTemplate(text string, c Context) ([]byte, error) {
	functions := template.FuncMap{"has": slices.Contains[[]string, string], "hasPrefix": strings.HasPrefix}
	parsed, err := template.New("asset").Funcs(functions).Option("missingkey=error").Parse(text)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}
	for _, item := range parsed.Templates() {
		if err := validateReferences(item.Root, c, ""); err != nil {
			return nil, err
		}
	}
	var output bytes.Buffer
	if err := parsed.Execute(&output, c); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}
	data := output.Bytes()
	if bytes.Contains(data, []byte("{{")) || bytes.Contains(data, []byte("}}")) {
		return nil, fmt.Errorf("%w: unresolved template delimiter", ErrRender)
	}
	return data, nil
}
