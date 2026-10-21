package assets

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
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
		return Rendered{}, fmt.Errorf("%w %q", errDestination, path)
	}
	body, err := fs.ReadFile(source, t.Source)
	if err != nil {
		return Rendered{}, fmt.Errorf("read template: %w", err)
	}
	allowCI := t.Source == ciSource && path == ciDestination
	data, err := executeAsset(string(body), c, allowCI)
	if err != nil {
		return Rendered{}, err
	}
	if err := parseRendered(path, data); err != nil {
		return Rendered{}, err
	}
	return Rendered{Template: t, Path: path, Data: data, SHA256: fmt.Sprintf("%x", sha256.Sum256(data))}, nil
}

func executeTemplate(text string, c Context) ([]byte, error) {
	return executeAsset(text, c, false)
}
