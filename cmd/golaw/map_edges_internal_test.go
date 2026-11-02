package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// TestMapNamesEverySeamThatFails keeps each failure of the command a
// failure of map with its cause, never a map printed from half a read.
func TestMapNamesEverySeamThatFails(t *testing.T) {
	t.Parallel()
	discover, root := mapApp(t, proof.Policy())
	discover.discover = func(context.Context, string) (string, error) { return "", errBoom }
	load, _ := mapApp(t, proof.Policy())
	load.load = func(context.Context, string, time.Time) (*model.EffectivePolicy, error) { return nil, errBoom }
	outside, _ := mapApp(t, proof.Policy())
	outside.discover = func(context.Context, string) (string, error) { return t.TempDir(), nil }
	for name, a := range map[string]testApp{"discover": discover, "load": load, "outside a repository": outside} {
		err := a.printMap(t.Context(), a.stdout, root, mapOptions{budget: 10})
		if err == nil || errors.Is(err, model.ErrUsage) || a.stdout.Len() != 0 {
			t.Errorf("%s: err = %v, out = %q", name, err, a.stdout.String())
		}
	}
	write, root := mapApp(t, proof.Policy())
	for _, asJSON := range []bool{false, true} {
		err := write.printMap(t.Context(), &doctorFailWriter{}, root, mapOptions{budget: 10, asJSON: asJSON})
		if !errors.Is(err, errBoom) {
			t.Errorf("a failing output (json %t): err = %v", asJSON, err)
		}
	}
}
