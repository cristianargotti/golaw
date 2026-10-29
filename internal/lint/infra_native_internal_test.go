package lint

import (
	"path/filepath"
	"reflect"
	"testing"

	checksurface "github.com/cristianargotti/golaw/internal/checks/surface"
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

const (
	// infraNativeTool names a row carried by a native golaw check.
	infraNativeTool = "golaw"
	// nativeManifest is the proof manifest of the native surface family.
	nativeManifest = "../proof/surface.yml"
)

// infraNativeRows answers the rows a native check carries.
func infraNativeRows() []Row {
	all := infraRows()
	rows := make([]Row, 0, len(all))
	for _, r := range all {
		if r.Tool == infraNativeTool {
			rows = append(rows, r)
		}
	}
	return rows
}

func TestInfraNativeRowsNameTheNativeChecks(t *testing.T) {
	t.Parallel()
	row := func(sonar, rule, match string, where ...string) Row {
		return Row{Sonar: sonar, Tool: infraNativeTool, Rule: rule, Where: where, Match: match, Set: model.SurfaceParity}
	}
	want := []Row{
		row("cloudformation:S6249", "cfn.bucket-https-only", "close", "N", "L"),
		row("docker:S6471", "docker.final-user", "close", "H", "N"),
		row("docker:S6472", "surface.hardcoded-credential", "close", "H", "N", "L"),
		row("docker:S8431", "docker.tag-and-digest", "exact", "H", "N", "L"),
	}
	if got := infraNativeRows(); !reflect.DeepEqual(got, want) {
		t.Errorf("native rows = %+v\nwant %+v", got, want)
	}
}

// TestInfraNativeRowsHoldAProvenCheck: each native row names a registered
// check of the surface family whose proof carries a red and a near tree.
func TestInfraNativeRowsHoldAProvenCheck(t *testing.T) {
	t.Parallel()
	m, err := proof.LoadManifest(nativeManifest)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, c := range checksurface.Checks() {
		ids[c.Spec().ID] = true
	}
	for _, r := range infraNativeRows() {
		if !ids[r.Rule] {
			t.Errorf("row %s names %s, which the surface family does not register", r.Sonar, r.Rule)
		}
		if e, ok := m.Lookup(r.Rule); !ok || e.Red == "" || e.Near == "" {
			t.Errorf("row %s: %s has no red and near-miss proof in %s", r.Sonar, r.Rule, nativeManifest)
		}
	}
}

// TestInfraNativeRedFixturesReportTheRowsSonarKey runs the red tree of each
// Dockerfile and CloudFormation check and reads the Sonar key its findings
// carry; surface.hardcoded-credential is left out because its red tree may
// be in any of its languages, each with its own key.
func TestInfraNativeRedFixturesReportTheRowsSonarKey(t *testing.T) {
	t.Parallel()
	m, err := proof.LoadManifest(nativeManifest)
	if err != nil {
		t.Fatal(err)
	}
	reg, err := proof.Registry(checksurface.Checks(), m)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range infraNativeRows() {
		if r.Rule == "surface.hardcoded-credential" {
			continue
		}
		t.Run(r.Rule, func(t *testing.T) {
			t.Parallel()
			infraNativeSonar(t, reg, m, r)
		})
	}
}

// infraNativeSonar dispatches the red tree of a row's check and compares
// the Sonar key of each finding of its code with the row's.
func infraNativeSonar(t *testing.T, reg *engine.Registry, m proof.Manifest, r Row) {
	t.Helper()
	e, ok := m.Lookup(r.Rule)
	if !ok {
		t.Fatalf("%s has no proof entry", r.Rule)
	}
	check, ok := reg.Lookup(r.Rule)
	if !ok {
		t.Fatalf("%s is not registered", r.Rule)
	}
	fixture, err := proof.LoadFixture(filepath.Join(filepath.Dir(nativeManifest), e.Red))
	if err != nil {
		t.Fatal(err)
	}
	in, err := fixture.Input(check.Spec().Scope, proof.PolicyFor(e))
	if err != nil {
		t.Fatal(err)
	}
	run, err := engine.Dispatch(t.Context(), reg, in)
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	findings := run.Findings()
	for i := range findings {
		f := &findings[i]
		if f.Check != r.Rule || f.Code != e.Code {
			continue
		}
		found++
		if f.Sonar != r.Sonar {
			t.Errorf("%s reports %q at %s:%d, the row says %q", r.Rule, f.Sonar, f.Path, f.Line, r.Sonar)
		}
	}
	if found == 0 {
		t.Errorf("the red tree of %s yields no %s", r.Rule, e.Code)
	}
}
