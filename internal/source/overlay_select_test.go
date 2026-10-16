package source_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/source"
)

// TestOverlaySelectsTheFoldedUniverse answers a part of the overlay with
// the candidates folded in, the way Files answers the whole: a removed
// path leaves, a replaced path keeps one entry, a created path joins in
// path order, and presence follows the same universe.
func TestOverlaySelectsTheFoldedUniverse(t *testing.T) {
	t.Parallel()
	base := &tree{files: map[string]string{"a/one.go": "1", "a/two.go": "2", "b/three.go": "3"}}
	o, _, err := source.NewOverlay(base, []source.Mutation{
		{Path: "a/two.go", Op: model.CandidateDelete},
		{Path: "a/one.go", Op: model.CandidateReplace, Content: []byte("new")},
		{Path: "a/zero.go", Op: model.CandidateCreate, Content: []byte("0")},
	})
	if err != nil {
		t.Fatal(err)
	}
	under := func(dir string) func(string) bool {
		return func(p string) bool { return strings.HasPrefix(p, dir+"/") }
	}
	selected := o.Select(under("a"))
	paths := make([]string, 0, len(selected))
	for _, f := range selected {
		paths = append(paths, f.Path)
		if f.Path == "a/one.go" && f.Size != 3 {
			t.Fatalf("the replaced entry = %+v", f)
		}
	}
	if !slices.Equal(paths, []string{"a/one.go", "a/zero.go"}) {
		t.Fatalf("Select = %v", paths)
	}
	whole := o.Files()
	if len(whole) != 3 || whole[2].Path != "b/three.go" || !slices.Equal(whole, o.Files()) {
		t.Fatalf("Files = %+v", whole)
	}
	if !o.Any(under("b")) || !o.Any(func(p string) bool { return p == "a/zero.go" }) || o.Any(func(p string) bool { return p == "a/two.go" }) {
		t.Fatal("Any must follow the folded universe")
	}
}
