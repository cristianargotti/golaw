package surface_test

import (
	"maps"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/surface"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
	"github.com/cristianargotti/golaw/internal/surfacefix"
)

// native tells the ids of this family from the lint tools of an
// expectation.
func native(id string) bool {
	return strings.HasPrefix(id, "surface.") || strings.HasPrefix(id, "docker.") || strings.HasPrefix(id, "cfn.")
}

// nativeCounts runs every check of the family over a twin in a scan under
// the strict set and counts its findings by check, path and code.
func nativeCounts(t *testing.T, twin surfacefix.Twin) map[string]int {
	t.Helper()
	files, err := surfacefix.Files(twin)
	if err != nil {
		t.Fatal(err)
	}
	text := make(map[string]string, len(files))
	for path, data := range files {
		text[path] = string(data)
	}
	strict := *proof.Policy()
	strict.Surface.Rules = model.SurfaceStrict
	got := map[string]int{}
	for _, c := range surface.Checks() {
		in := scanInput(c, text)
		in.Policy = &strict
		res, err := c.Run(t.Context(), in)
		if err != nil || res.Status.Kind == model.StatusUnmeasured {
			t.Fatalf("%s: %+v, %v", c.Spec().ID, res.Status, err)
		}
		for i := range res.Findings {
			got[c.Spec().ID+" "+res.Findings[i].Path+" "+res.Findings[i].Code]++
		}
	}
	return got
}

func TestTheRedTwinRaisesTheNativeClassesAndTheCorrectedTwinNone(t *testing.T) {
	t.Parallel()
	want := map[string]int{}
	for _, e := range surfacefix.Expectations() {
		if native(e.Check) {
			want[e.Check+" "+e.Path+" "+e.Code] += e.Count
		}
	}
	if got := nativeCounts(t, surfacefix.Red); !maps.Equal(got, want) {
		t.Errorf("red twin = %v\nwant %v", got, want)
	}
	if got := nativeCounts(t, surfacefix.Corrected); len(got) != 0 {
		t.Errorf("corrected twin = %v, want no native finding", got)
	}
}
