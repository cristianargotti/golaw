package newcode_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/newcode"
)

// readSource is the precedent's source: a blank line inside the first
// function, which the line model drops, and a second function.
const readSource = "package example\nfunc read() {\n\tfirst()\n\n\tsecond()\n}\nfunc other() {\n\tthird()\n}\n"

// checkout answers the bytes of a module-relative path the way a snapshot
// does, ErrNotInSnapshot for a path it does not hold.
func checkout(files map[string]string) func(string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		data, ok := files[path]
		if !ok {
			return nil, model.ErrNotInSnapshot
		}
		return []byte(data), nil
	}
}

func lineFiles() map[string]string {
	return map[string]string{
		"read.go":      readSource,
		"crlf/crlf.go": "package crlf\r\nfunc f() {\r\n\tg()\r\n}\r\n",
	}
}

func TestLinesFollowSonarsLineModel(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		profile string
		want    map[string]newcode.LineHits
	}{
		{
			"near-hits-merge-per-line-and-blank-lines-drop",
			"mode: set\nexample/read.go:2.13,6.2 2 0\nexample/read.go:2.13,6.2 2 1\nexample/read.go:7.14,9.2 1 0\n",
			map[string]newcode.LineHits{"read.go": {3: true, 5: true, 8: false}},
		},
		{
			"near-count-mode",
			"mode: count\nexample/read.go:7.14,9.2 1 4\n",
			map[string]newcode.LineHits{"read.go": {8: true}},
		},
		{
			"near-atomic-mode-and-a-crlf-report",
			"mode: atomic\r\nexample/read.go:7.14,9.2 1 0\r\n\r\n",
			map[string]newcode.LineHits{"read.go": {8: false}},
		},
		{
			"near-a-crlf-source",
			"mode: set\nexample/crlf/crlf.go:2.10,4.2 1 1\n",
			map[string]newcode.LineHits{"crlf/crlf.go": {3: true}},
		},
		{
			"near-a-report-of-no-block",
			"mode: set\n",
			map[string]newcode.LineHits{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := newcode.Lines(strings.NewReader(tc.profile), "example", checkout(lineFiles()))
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Lines = %v, %v, want %v", got, err, tc.want)
			}
		})
	}
}

func TestLinesRefuseAReportTheyCannotMeasure(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		profile string
	}{
		{"red-empty", ""},
		{"red-no-mode-line", "example/read.go:2.13,6.2 2 1\n"},
		{"red-an-unknown-mode", "mode: sometimes\n"},
		{"red-a-garbage-block", "mode: set\nbad\n"},
		{"red-missing-fields", "mode: set\nexample/read.go:2.13,6.2 2\n"},
		{"red-bad-coordinates", "mode: set\nexample/read.go:2.x,6.2 2 1\n"},
		{"red-a-reversed-block", "mode: set\nexample/read.go:6.1,2.1 2 1\n"},
		{"red-negative-hits", "mode: set\nexample/read.go:2.13,6.2 2 -1\n"},
		{"red-negative-statements", "mode: set\nexample/read.go:2.13,6.2 -2 1\n"},
		{"red-outside-the-module", "mode: set\nother/read.go:2.13,6.2 2 1\n"},
		{"red-an-escaping-path", "mode: set\nexample/../read.go:2.13,6.2 2 1\n"},
		{"red-a-source-the-checkout-lacks", "mode: set\nexample/missing.go:1.1,2.1 1 0\n"},
		{"red-a-line-past-the-end", "mode: set\nexample/read.go:40.1,41.2 1 0\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := newcode.Lines(strings.NewReader(tc.profile), "example", checkout(lineFiles()))
			if !errors.Is(err, newcode.ErrReport) || got != nil {
				t.Fatalf("Lines = %v, %v, want ErrReport", got, err)
			}
		})
	}
}

func TestLinesKeepTheCauseOfAnUnreadableSource(t *testing.T) {
	t.Parallel()
	profile := "mode: set\nexample/missing.go:1.1,2.1 1 0\n"
	_, err := newcode.Lines(strings.NewReader(profile), "example", checkout(lineFiles()))
	if !errors.Is(err, model.ErrNotInSnapshot) || !strings.Contains(err.Error(), "missing.go") {
		t.Fatalf("error = %v, want the path and the read failure", err)
	}
}

func TestModuleNamesTheProfilePrefix(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		gomod string
		want  string
		err   error
	}{
		{"near-plain", "module example.test/m\n\ngo 1.27\n", "example.test/m", nil},
		{"near-quoted-with-a-comment", "// a sample\nmodule \"example.test/q\" // quoted\n", "example.test/q", nil},
		{"red-no-module-line", "go 1.27\n", "", newcode.ErrModule},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := newcode.Module([]byte(tc.gomod))
			if got != tc.want || !errors.Is(err, tc.err) {
				t.Fatalf("Module = %q, %v, want %q, %v", got, err, tc.want, tc.err)
			}
		})
	}
}
