package conformance_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/surfacefix"
)

func TestTheInventoryOfTheTwinsHoldsEveryHubLanguage(t *testing.T) {
	t.Parallel()
	languages := []surface.Language{
		surface.JavaScript, surface.JSX, surface.HTML, surface.Python, surface.Dockerfile, surface.CloudFormation, surface.Shell,
	}
	for name, which := range map[string]surfacefix.Twin{"the red twin": surfacefix.Red, "the corrected twin": surfacefix.Corrected} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			inv, err := surface.Take(t.Context(), twin(t, which))
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(inv.Languages, languages) || len(inv.Unmeasured) != 0 {
				t.Fatalf("Languages = %v, Unmeasured = %v, want %v and none", inv.Languages, inv.Unmeasured, languages)
			}
			for _, f := range inv.Files {
				if f.Class != surface.ClassSource {
					t.Errorf("%s is %s: every file of a twin is hand written source", f.Path, f.Class)
				}
			}
		})
	}
}

func TestTheRedViewHoldsAnInlineScriptAndAStyle(t *testing.T) {
	t.Parallel()
	red := twin(t, surfacefix.Red)
	info, err := surface.Detect("views/panel.html", red["views/panel.html"])
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Regions) != 2 || info.Regions[0].Kind != surface.RegionStyle || info.Regions[1].Kind != surface.RegionScript {
		t.Fatalf("regions = %+v, want the style then the inline script", info.Regions)
	}
	if script := info.Regions[1]; script.Lines() <= 40 || script.Spliced || script.Module {
		t.Errorf("the inline script spans %d lines, want more than the 40 of surface.inline-script-size", script.Lines())
	}
	corrected, err := surface.Detect("views/panel.html", twin(t, surfacefix.Corrected)["views/panel.html"])
	if err != nil || len(corrected.Regions) != 1 || corrected.Regions[0].Kind != surface.RegionStyle {
		t.Fatalf("the corrected view loads its script with src: %+v, %v", corrected.Regions, err)
	}
}
