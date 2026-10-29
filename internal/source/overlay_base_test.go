package source_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/source"
)

// TestOverlayReadsItsBaseApart proves what a native check of M6 relies on to
// deny only what a candidate introduces: the overlay answers the candidate,
// its base answers the bytes on disk, and a path the candidate creates is
// absent from the base.
func TestOverlayReadsItsBaseApart(t *testing.T) {
	t.Parallel()
	disk := &tree{files: map[string]string{"views/panel.js": "legacy();\n"}}
	overlay, _, err := source.NewOverlay(disk, []source.Mutation{
		{Path: "views/panel.js", Op: model.CandidateReplace, Content: []byte("rewritten();\n")},
		{Path: "views/new.js", Op: model.CandidateCreate, Content: []byte("fresh();\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	var snapshot model.Snapshot = overlay
	based, ok := snapshot.(model.Based)
	if !ok {
		t.Fatal("the overlay is not a model.Based snapshot")
	}
	tests := map[string]struct {
		path    string
		overlay string
		base    string
		held    bool
	}{
		"a replaced file reads its candidate and its base apart": {path: "views/panel.js", overlay: "rewritten();\n", base: "legacy();\n", held: true},
		"a created file is absent from the base (near miss)":     {path: "views/new.js", overlay: "fresh();\n", held: false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got, err := overlay.Read(tc.path); err != nil || string(got) != tc.overlay {
				t.Fatalf("overlay Read(%s) = %q, %v, want %q", tc.path, got, err, tc.overlay)
			}
			held, err := based.Base().Read(tc.path)
			switch {
			case !tc.held && err == nil:
				t.Errorf("base Read(%s) = %q, want an error: the base never sees a candidate", tc.path, held)
			case tc.held && (err != nil || string(held) != tc.base):
				t.Errorf("base Read(%s) = %q, %v, want %q", tc.path, held, err, tc.base)
			}
		})
	}
}
