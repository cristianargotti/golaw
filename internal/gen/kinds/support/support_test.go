package support_test

import (
	"bytes"
	"context"
	"errors"
	"go/parser"
	"go/token"
	"path"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/kinds/support"
	"github.com/cristianargotti/golaw/internal/genheader"
	"github.com/cristianargotti/golaw/internal/model"
)

func input() genkind.Input {
	return genkind.Input{Module: "example.com/slice", Version: "0.3.0", Policy: model.DefaultGenerate()}
}

func TestSupportRendersTheCorePackageWithHeaders(t *testing.T) {
	t.Parallel()
	k := support.Kind()
	if k.Name() != genkind.KindSupport || k.Template() != 1 {
		t.Fatalf("kind = %s@%d", k.Name(), k.Template())
	}
	out, err := k.New(t.Context(), input(), genkind.Request{})
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, len(out.Regenerable))
	for _, f := range out.Regenerable {
		paths = append(paths, f.Path)
		requireSupportFile(t, f)
	}
	for _, want := range []string{"errors", "errors_test", "hole", "hole_test", "recorder", "recorder_test"} {
		if !slices.Contains(paths, "internal/gensupport/zz_generated_"+want+".go") {
			t.Errorf("paths = %v, want zz_generated_%s.go", paths, want)
		}
	}
	want := genkind.Output{}.Entry
	want.Kind, want.Template, want.Name, want.Regenerable = genkind.KindSupport, "support@1", "gensupport.Support", paths
	if !slices.IsSorted(paths) || !reflect.DeepEqual(out.Entry, want) || len(out.Owned) != 0 || len(out.Appends) != 0 {
		t.Errorf("entry = %+v, owned %d, appends %d", out.Entry, len(out.Owned), len(out.Appends))
	}
	again, err := k.Regenerate(t.Context(), input(), out.Entry)
	if err != nil || !reflect.DeepEqual(again, out) {
		t.Errorf("Regenerate differs from New: %v", err)
	}
}

func requireSupportFile(t *testing.T, f genkind.File) {
	t.Helper()
	h, ok := genheader.Parse(f.Content)
	if !ok || h != (genheader.Header{Version: "0.3.0", Kind: genkind.KindSupport, Template: 1}) {
		t.Errorf("%s header = %+v, %v", f.Path, h, ok)
	}
	file, err := parser.ParseFile(token.NewFileSet(), f.Path, f.Content, parser.PackageClauseOnly)
	if err != nil {
		t.Fatalf("%s does not parse: %v", f.Path, err)
	}
	wantPackage := path.Base(path.Dir(f.Path))
	// An internal test shares its package, so only an external one is _test.
	if strings.HasSuffix(f.Path, "_test.go") && !strings.HasSuffix(f.Path, "_internal_test.go") {
		wantPackage += "_test"
	}
	if file.Name.Name != wantPackage || bytes.Contains(f.Content, []byte("\r")) {
		t.Errorf("%s package %s, want %s, or a carriage return", f.Path, file.Name.Name, wantPackage)
	}
}

func TestSupportFollowsTheConfiguredPath(t *testing.T) {
	t.Parallel()
	in := input()
	in.Policy.Support = "internal/platform/support"
	out, err := support.Kind().New(t.Context(), in, genkind.Request{})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range out.Regenerable {
		if !strings.HasPrefix(f.Path, "internal/platform/support/") {
			t.Errorf("path %s outside the configured support path", f.Path)
		}
		requireSupportFile(t, f)
	}
	if out.Entry.Name != "support.Support" {
		t.Errorf("entry name = %s", out.Entry.Name)
	}
}

// Red rows: a support package needs a module and a path, and a canceled
// run renders nothing.
func TestSupportRefusesWhatItCannotRender(t *testing.T) {
	t.Parallel()
	noModule, noPath := input(), input()
	noModule.Module, noPath.Policy.Support = "", ""
	for name, in := range map[string]genkind.Input{"no module": noModule, "no support path": noPath} {
		if _, err := support.Kind().New(t.Context(), in, genkind.Request{}); !errors.Is(err, genkind.ErrRequest) {
			t.Errorf("%s = %v, want ErrRequest", name, err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := support.Kind().New(ctx, input(), genkind.Request{}); !errors.Is(err, context.Canceled) {
		t.Errorf("a cancelled run = %v", err)
	}
}
