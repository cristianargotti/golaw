package goldentest_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/goldentest"
)

func TestLoadFillsReadsTheRecordedFills(t *testing.T) {
	t.Parallel()
	got, err := goldentest.LoadFills("testdata/fills/good.yml")
	if err != nil {
		t.Fatal(err)
	}
	want := []goldentest.Fill{
		{ID: "greeter.greet", Body: "return g.prefix + \" \" + name, ctx.Err()\n"},
		{ID: "welcomer.greet", Body: "return strings.Join([]string{g.prefix, name}, \" \"), ctx.Err()\n", Imports: []genkind.Import{{Path: "strings"}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadFills = %+v, want %+v", got, want)
	}
}

func TestLoadFillsRefusesARecordItCannotTrust(t *testing.T) {
	t.Parallel()
	tests := map[string]struct{ path, want string }{
		"red: an unknown key": {"testdata/fills/unknown-key.yml", "field when not found"},
		"red: a duplicate id": {"testdata/fills/duplicate.yml", "duplicate fill greeter.greet"},
		"red: an empty body":  {"testdata/fills/empty-body.yml", "fill greeter.greet has no body"},
		"red: no file":        {"testdata/fills/absent.yml", "absent.yml"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := goldentest.LoadFills(tc.path)
			if !errors.Is(err, goldentest.ErrFills) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("LoadFills(%s) = %v, want ErrFills holding %q", tc.path, err, tc.want)
			}
		})
	}
}
