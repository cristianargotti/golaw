package hooktest

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestRouteRefusesAFixtureItCannotRead(t *testing.T) {
	t.Parallel()
	if _, _, _, _, err := split(proof.NewTree(map[string][]byte{"notes.md": []byte("x")})); !errors.Is(err, proof.ErrFixture) {
		t.Fatalf("a fixture without a payload was read: %v", err)
	}
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	r := route{id: "probe.route", code: "route", agent: "probe", state: blocked}
	in := model.Input{Mode: model.ModeScan, Snapshot: proof.NewTree(map[string][]byte{PayloadFile: []byte("{}")})}
	if _, err := r.Run(t.Context(), in); err == nil {
		t.Fatal("a route ran without a state directory")
	}
	if r.Spec().Rule != routeRule || r.Spec().Scope != model.ScopeTree {
		t.Fatalf("%+v", r.Spec())
	}
}
