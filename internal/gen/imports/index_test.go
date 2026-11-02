package imports_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/imports"
)

func TestIndexAnswersTheStandardLibraryAndTheModule(t *testing.T) {
	t.Parallel()
	std := imports.Std()
	if name, ok := std.Name("net/http"); !ok || name != "http" {
		t.Errorf("Name(net/http) = %q, %v", name, ok)
	}
	if got := std.Lookup("template"); !slices.Equal(got, []string{"html/template", "text/template"}) {
		t.Errorf("Lookup(template) = %v", got)
	}
	if got := std.Lookup("abi"); len(got) != 0 {
		t.Errorf("an internal package entered the index: %v", got)
	}
	x := moduleIndex()
	if got := x.Lookup("growth"); !slices.Equal(got, []string{"example.com/slice/internal/application/growth"}) {
		t.Errorf("Lookup(growth) = %v", got)
	}
	if got := x.Lookup("release"); !slices.Equal(got, []string{"example.com/slice/internal/version"}) {
		t.Errorf("Lookup(release) = %v", got)
	}
	if got := x.Lookup("main"); len(got) != 0 {
		t.Errorf("a main package entered the index: %v", got)
	}
	if got := x.Lookup("fixture"); len(got) != 0 {
		t.Errorf("a testdata package entered the index: %v", got)
	}
	if got := std.Lookup("growth"); len(got) != 0 {
		t.Errorf("With changed its receiver: %v", got)
	}
}
