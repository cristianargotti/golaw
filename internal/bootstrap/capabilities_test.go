package bootstrap_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/model"
)

// plainLayout is the layout of a repository without a go.mod at the root.
func plainLayout() bootstrap.Layout {
	return bootstrap.Layout{Main: bootstrap.MainRoot, Decisions: "docs/adr"}
}

func TestNormalizeRefusesAGoShapeOutsideAGoModule(t *testing.T) {
	t.Parallel()
	in := bootstrap.Inputs{KitVersion: "0.1.0", Language: "pt-BR", Capabilities: []string{"image", "hexagonal"}, Layout: plainLayout()}
	_, err := in.Normalize()
	want := "usage: invalid policy: capability hexagonal needs go: add go to the capabilities"
	if !errors.Is(err, model.ErrUsage) || !errors.Is(err, model.ErrPolicy) || err.Error() != want {
		t.Fatalf("Normalize = %v, want %q", err, want)
	}
}

func TestNormalizeKeepsEverySetThatHoldsTheFloor(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		in   bootstrap.Inputs
		want []string
	}{
		"a Go module adds go": {bootstrap.Inputs{Capabilities: []string{"cli"}, Layout: goLayout()}, []string{"cli", "go"}},
		"go named outside":    {bootstrap.Inputs{Capabilities: []string{"api", "go"}, Layout: plainLayout()}, []string{"api", "go"}},
		"image outside":       {bootstrap.Inputs{Capabilities: []string{"image"}, Layout: plainLayout()}, []string{"image"}},
		"nothing outside":     {bootstrap.Inputs{Layout: plainLayout()}, []string{}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tc.in.KitVersion, tc.in.Language = "0.1.0", "pt-BR"
			got, err := tc.in.Normalize()
			if err != nil || !slices.Equal(got.Capabilities, tc.want) {
				t.Fatalf("Normalize = %v, %v, want %v", got.Capabilities, err, tc.want)
			}
		})
	}
}
