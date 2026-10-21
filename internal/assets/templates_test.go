package assets_test

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/cristianargotti/golaw/internal/assets"
)

func TestTemplatesValidateTheManifest(t *testing.T) {
	t.Parallel()
	const entry = `templates:
  - source: templates/a.yml.tmpl
    destination: a.yml
    ownership: kit
    when: []
`
	cases := []struct {
		name, document string
		removeSource   bool
		fail           bool
	}{
		{"kit", entry, false, false},
		{"merged", strings.Replace(entry, "ownership: kit", "ownership: merged", 1), false, false},
		{"developer", strings.Replace(entry, "ownership: kit", "ownership: developer", 1), false, false},
		{"capabilities", strings.Replace(entry, "when: []", "when: [go, db]", 1), false, false},
		{"decisions", strings.Replace(entry, "destination: a.yml",
			`destination: "{{.Decisions}}/record.md"`, 1), false, false},
		{"unknown key", entry + "unknown: true\n", false, true},
		{"duplicate key", entry + "templates: []\n", false, true},
		{"second document", entry + "---\ntemplates: []\n", false, true},
		{"missing source", entry, true, true},
		{"source outside", strings.Replace(entry, "templates/a.yml.tmpl", "../a.yml.tmpl", 1), false, true},
		{"source suffix", strings.Replace(entry, "templates/a.yml.tmpl", "templates/a.yml", 1), false, true},
		{"destination outside", strings.Replace(entry, "destination: a.yml", "destination: ../a.yml", 1), false, true},
		{"absolute destination", strings.Replace(entry, "destination: a.yml", "destination: /a.yml", 1), false, true},
		{"ownership", strings.Replace(entry, "ownership: kit", "ownership: shared", 1), false, true},
		{"capability", strings.Replace(entry, "when: []", "when: [unknown]", 1), false, true},
		{"duplicate destination", entry + strings.TrimPrefix(entry, "templates:\n"), false, true},
		{"unsorted", strings.Replace(entry, "destination: a.yml", "destination: z.yml", 1) +
			strings.TrimPrefix(entry, "templates:\n"), false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tree := fstest.MapFS{
				"templates/manifest.yml": {Data: []byte(tc.document)},
				"templates/a.yml.tmpl":   {Data: []byte("value: true\n")},
				"templates/a.yml":        {Data: []byte("value: true\n")},
			}
			if tc.removeSource {
				delete(tree, "templates/a.yml.tmpl")
			}
			got, err := assets.Templates(tree)
			if tc.fail {
				if !errors.Is(err, assets.ErrManifest) {
					t.Fatalf("expected manifest error, got %v", err)
				}
				return
			}
			if err != nil || len(got) != 1 {
				t.Fatalf("templates = %+v, %v", got, err)
			}
		})
	}
}

const (
	templateTestManifest = "templates/manifest.yml"
	templateTestSource   = "templates/extra.tmpl"
	templateTestEntry    = "  - source: " + templateTestSource + "\n" +
		"    destination: output.yml\n    ownership: kit\n    when: []\n"
)

func TestTemplatesRejectInvalidDocumentShapes(t *testing.T) {
	t.Parallel()
	cases := []string{
		"", "[", "null\n", "{}\n", "templates: null\n", "templates: {}\n",
		"templates: [null]\n", "templates: [1]\n", "templates: []\n---\n",
		"templates:\n" + templateTestEntry + "    unknown: true\n",
		"templates:\n" + templateTestEntry + "    ownership: merged\n",
		"templates:\n" + strings.Replace(templateTestEntry, "when: []", "when: [null]", 1),
		"templates:\n" + strings.Replace(templateTestEntry, "when: []", "when: [go, null]", 1),
	}
	for _, document := range cases {
		t.Run(document, func(t *testing.T) {
			t.Parallel()
			_, err := assets.Templates(fstest.MapFS{
				templateTestManifest: {Data: []byte(document)},
				templateTestSource:   {Data: []byte("value\n")},
			})
			if !errors.Is(err, assets.ErrManifest) {
				t.Fatalf("expected manifest error, got %v", err)
			}
		})
	}
	if _, err := assets.Templates(fstest.MapFS{}); !errors.Is(err, assets.ErrManifest) {
		t.Fatalf("missing manifest: %v", err)
	}
}

func TestTemplatesValidatePathsAndSourceReads(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, field, value string
		directory          bool
	}{
		{"empty destination", "destination", "''", false},
		{"root destination", "destination", "'.'", false},
		{"dot destination", "destination", "'./file.yml'", false},
		{"parent destination", "destination", "'a/../file.yml'", false},
		{"empty segment", "destination", "'a//file.yml'", false},
		{"backslash destination", "destination", "'a\\file.yml'", false},
		{"drive destination", "destination", "'C:/file.yml'", false},
		{"drive relative destination", "destination", "'C:file.yml'", false},
		{"unknown placeholder", "destination", "'{{.Unknown}}/file.yml'", false},
		{"unfinished placeholder", "destination", "'{{.Decisions/file.yml'", false},
		{"trailing delimiter", "destination", "'file}}.yml'", false},
		{"placeholder parent", "destination", "'{{.Decisions}}/../file.yml'", false},
		{"source outside templates", "source", "'other/extra.tmpl'", false},
		{"source backslash", "source", "'templates/a\\extra.tmpl'", false},
		{"source parent", "source", "'templates/../extra.tmpl'", false},
		{"directory source", "source", templateTestSource, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			entry := strings.Replace(templateTestEntry, "source: "+templateTestSource, "source: "+tc.value, 1)
			if tc.field == "destination" {
				entry = strings.Replace(templateTestEntry, "destination: output.yml", "destination: "+tc.value, 1)
			}
			tree := fstest.MapFS{
				templateTestManifest: {Data: []byte("templates:\n" + entry)},
				templateTestSource:   {Data: []byte("value\n")},
			}
			if tc.directory {
				delete(tree, templateTestSource)
				tree[templateTestSource+"/child"] = &fstest.MapFile{}
			}
			if _, err := assets.Templates(tree); !errors.Is(err, assets.ErrManifest) {
				t.Fatalf("expected manifest error, got %v", err)
			}
		})
	}
}

func TestTemplatesPreserveSortedEntriesAndEmptyManifests(t *testing.T) {
	t.Parallel()
	tree := fstest.MapFS{templateTestManifest: {Data: []byte("templates: []\n")}}
	if got, err := assets.Templates(tree); err != nil || len(got) != 0 {
		t.Fatalf("empty manifest = %+v, %v", got, err)
	}
	destinations := []string{".config/file.yml", "docs/record.md", "{{.Decisions}}/record.md"}
	entries := make([]string, 0, len(destinations))
	for _, destination := range destinations {
		entry := strings.Replace(templateTestEntry, "output.yml", "'"+destination+"'", 1)
		entries = append(entries, strings.Replace(entry, "when: []", "when: [go, db]", 1))
	}
	tree[templateTestManifest].Data = []byte("templates:\n" + strings.Join(entries, ""))
	tree[templateTestSource] = &fstest.MapFile{}
	got, err := assets.Templates(tree)
	if err != nil || len(got) != len(destinations) {
		t.Fatalf("sorted manifest = %+v, %v", got, err)
	}
	for i, entry := range got {
		if entry.Source != templateTestSource || entry.Destination != destinations[i] ||
			entry.Ownership != assets.OwnedByKit || strings.Join(entry.When, ",") != "go,db" {
			t.Fatalf("entry %d changed: %+v", i, entry)
		}
	}
}
