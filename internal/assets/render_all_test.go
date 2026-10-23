package assets_test

import (
	"errors"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/cristianargotti/golaw/internal/assets"
)

func TestRenderAllSelectsAndSortsExpandedDestinations(t *testing.T) {
	t.Parallel()
	const manifest = `templates:
  - source: templates/a.tmpl
    destination: a.txt
    ownership: kit
    when: [go, db]
  - source: templates/b.tmpl
    destination: z.txt
    ownership: developer
    when: []
`
	tree := fstest.MapFS{
		"templates/manifest.yml": {Data: []byte(manifest)},
		"templates/a.tmpl":       {Data: []byte("database\n")},
		"templates/b.tmpl":       {Data: []byte("always\n")},
	}
	for _, tc := range []struct {
		name string
		caps []string
		want []string
	}{
		{"neither", nil, []string{"z.txt"}},
		{"one", []string{"go"}, []string{"z.txt"}},
		{"both", []string{"go", "db"}, []string{"a.txt", "z.txt"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			values := templateValues()
			values.Capabilities = tc.caps
			got, err := assets.RenderAllContext(tree, assets.Context{Values: values, CIBackend: "none"})
			if err != nil {
				t.Fatal(err)
			}
			paths := make([]string, 0, len(got))
			for _, file := range got {
				paths = append(paths, file.Path)
			}
			if !reflect.DeepEqual(paths, tc.want) {
				t.Fatalf("paths = %v", paths)
			}
		})
	}
	collision := fstest.MapFS{
		"templates/manifest.yml": {Data: []byte(`templates:
  - source: templates/a.tmpl
    destination: docs/adr/a.txt
    ownership: kit
    when: []
  - source: templates/b.tmpl
    destination: "{{.Decisions}}/a.txt"
    ownership: kit
    when: []
`)},
		"templates/a.tmpl": {Data: []byte("first\n")},
		"templates/b.tmpl": {Data: []byte("second\n")},
	}
	if _, err := assets.RenderAllContext(collision, assets.Context{Values: templateValues(), CIBackend: "none"}); !errors.Is(err, assets.ErrRender) {
		t.Fatalf("expanded collision = %v", err)
	}
}

func TestRenderAllContextPreservesTypedInputs(t *testing.T) {
	t.Parallel()
	tree := fstest.MapFS{
		"templates/manifest.yml": {Data: []byte(`templates:
  - source: templates/context.tmpl
    destination: context.txt
    ownership: kit
    when: []
`)},
		"templates/context.tmpl": {Data: []byte(`{{.CIBackend}}
{{range .DirectRequires}}{{.Path}} {{.Version}}
{{end}}`)},
	}
	context := assets.Context{
		Values: templateValues(), CIBackend: "pipeline",
		DirectRequires: []assets.Requirement{{Path: "example.test/dependency", Version: "v1.2.3"}},
	}
	files, err := assets.RenderAllContext(tree, context)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || string(files[0].Data) != "pipeline\nexample.test/dependency v1.2.3\n" {
		t.Fatalf("context files = %+v", files)
	}
	files, err = assets.RenderAllContext(tree, assets.Context{Values: templateValues(), CIBackend: "none"})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || string(files[0].Data) != "none\n" {
		t.Fatalf("default files = %+v", files)
	}
}

func TestRenderAllSortsAfterExpansion(t *testing.T) {
	t.Parallel()
	tree := fstest.MapFS{
		"templates/manifest.yml": {Data: []byte(`templates:
  - source: templates/z.tmpl
    destination: z.txt
    ownership: kit
    when: []
  - source: templates/a.tmpl
    destination: "{{.Decisions}}/a.txt"
    ownership: kit
    when: []
`)},
		"templates/z.tmpl": {Data: []byte("last\n")},
		"templates/a.tmpl": {Data: []byte("first\n")},
	}
	values := templateValues()
	values.Decisions = "a"
	files, err := assets.RenderAllContext(tree, assets.Context{Values: values, CIBackend: "none"})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[0].Path != "a/a.txt" || files[1].Path != "z.txt" {
		t.Fatalf("expanded order = %+v", files)
	}
	if string(files[0].Data) != "first\n" || files[0].Template.Source != "templates/a.tmpl" {
		t.Fatalf("first file = %+v", files[0])
	}
}

func TestRenderAllIgnoresUnselectedCollision(t *testing.T) {
	t.Parallel()
	tree := fstest.MapFS{
		"templates/manifest.yml": {Data: []byte(`templates:
  - source: templates/a.tmpl
    destination: docs/adr/a.txt
    ownership: kit
    when: [go]
  - source: templates/b.tmpl
    destination: "{{.Decisions}}/a.txt"
    ownership: kit
    when: [db]
`)},
		"templates/a.tmpl": {Data: []byte("selected\n")},
		"templates/b.tmpl": {Data: []byte("{{.Absent}}\n")},
	}
	files, err := assets.RenderAllContext(tree, assets.Context{Values: templateValues(), CIBackend: "none"})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "docs/adr/a.txt" || string(files[0].Data) != "selected\n" {
		t.Fatalf("selected files = %+v", files)
	}
	values := templateValues()
	values.Capabilities = nil
	files, err = assets.RenderAllContext(tree, assets.Context{Values: values, CIBackend: "none"})
	if err != nil || len(files) != 0 {
		t.Fatalf("unselected files = %+v, error = %v", files, err)
	}
}

func TestRenderAllEmptyManifest(t *testing.T) {
	t.Parallel()
	tree := fstest.MapFS{"templates/manifest.yml": {Data: []byte("templates: []\n")}}
	files, err := assets.RenderAllContext(tree, assets.Context{Values: templateValues(), CIBackend: "none"})
	if err != nil || len(files) != 0 {
		t.Fatalf("empty files = %+v, error = %v", files, err)
	}
}

func TestRenderAllPreservesErrors(t *testing.T) {
	t.Parallel()
	files, err := assets.RenderAllContext(fstest.MapFS{}, assets.Context{Values: templateValues(), CIBackend: "none"})
	if files != nil || !errors.Is(err, assets.ErrManifest) || !errors.Is(err, assets.ErrRender) {
		t.Fatalf("manifest files = %+v, error = %v", files, err)
	}
	tree := fstest.MapFS{
		"templates/manifest.yml": {Data: []byte(`templates:
  - source: templates/a.tmpl
    destination: a.txt
    ownership: kit
    when: []
  - source: templates/b.tmpl
    destination: b.txt
    ownership: kit
    when: []
`)},
		"templates/a.tmpl": {Data: []byte("valid\n")},
		"templates/b.tmpl": {Data: []byte("{{.Absent}}\n")},
	}
	files, err = assets.RenderAllContext(tree, assets.Context{Values: templateValues(), CIBackend: "none"})
	if files != nil || !errors.Is(err, assets.ErrRender) || errors.Is(err, assets.ErrManifest) {
		t.Fatalf("render files = %+v, error = %v", files, err)
	}
}
