package policyfiles

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestRecordModulesReadsOnlyInitialStringSequence(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		data string
		want []string
	}{
		{name: "empty"},
		{name: "prose", data: "modules: [example.test/a]\n"},
		{name: "later delimiter", data: "prose\n---\nmodules: [example.test/a]\n---\n"},
		{name: "empty front matter", data: "---\n---\n"},
		{name: "metadata only", data: "---\nrecord: 1\nstatus: accepted\n---\n"},
		{name: "empty sequence", data: "---\nmodules: []\n---\n"},
		{
			name: "block sequence and invalid prose",
			data: "---\nrecord: 1\nmodules:\n - example.test/a\n - example.test/b\n---\n[invalid YAML",
			want: []string{"example.test/a", "example.test/b"},
		},
		{
			name: "CRLF and no closing newline",
			data: "---\r\nmodules: [example.test/a]\r\n---\r",
			want: []string{"example.test/a"},
		},
		{
			name: "quoted strings remain exact",
			data: "---\nmodules: ['123', 'true', 'null', ' example.test/a ']\n---\n",
			want: []string{"123", "true", "null", " example.test/a "},
		},
		{
			name: "aliased strings",
			data: "---\nlibrary: &library example.test/a\nmodules: [*library]\n---\n",
			want: []string{"example.test/a"},
		},
		{
			name: "aliased sequence",
			data: "---\nlibraries: &libraries [example.test/a]\nmodules: *libraries\n---\n",
			want: []string{"example.test/a"},
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got, err := recordModules("docs/adr/library.md", []byte(row.data))
			if err != nil || !slices.Equal(got, row.want) {
				t.Fatalf("modules = %v, error %v, want %v", got, err, row.want)
			}
		})
	}
}

func TestRecordModulesRejectsUnsupportedShapes(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		data string
		line string
	}{
		{name: "unterminated", data: "---\nmodules: []\n", line: ":1:"},
		{name: "scalar", data: "---\nmodules: example.test/a\n---\n", line: ":2:"},
		{name: "mapping", data: "---\nmodules: {example.test/a: accepted}\n---\n", line: ":2:"},
		{name: "null", data: "---\nmodules:\n---\n", line: ":2:"},
		{name: "number", data: "---\nmodules: [123]\n---\n", line: ":2:"},
		{name: "boolean", data: "---\nmodules: [true]\n---\n", line: ":2:"},
		{name: "null element", data: "---\nmodules: [null]\n---\n", line: ":2:"},
		{name: "nested sequence", data: "---\nmodules: [[example.test/a]]\n---\n", line: ":2:"},
		{name: "mapping element", data: "---\nmodules: [{path: example.test/a}]\n---\n", line: ":2:"},
		{name: "root sequence", data: "---\n- example.test/a\n---\n", line: ":2:"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			modules, err := recordModules("docs/adr/library.md", []byte(row.data))
			if len(modules) != 0 || !errors.Is(err, model.ErrOutsideSubset) {
				t.Fatalf("modules = %v, error %v, want outside subset", modules, err)
			}
			if !strings.Contains(err.Error(), "docs/adr/library.md"+row.line) {
				t.Fatalf("error = %v, want original record location %s", err, row.line)
			}
		})
	}
}

func TestRecordModulesNamesMalformedYAML(t *testing.T) {
	t.Parallel()
	modules, err := recordModules("docs/adr/library.md", []byte("---\nmodules: [\n---\n"))
	if len(modules) != 0 || err == nil || errors.Is(err, model.ErrOutsideSubset) {
		t.Fatalf("modules = %v, error %v, want a parse failure", modules, err)
	}
	if !strings.HasPrefix(err.Error(), "parse docs/adr/library.md:") {
		t.Fatalf("error = %v, want the record path", err)
	}
}
