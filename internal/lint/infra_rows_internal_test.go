package lint

import (
	"reflect"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// infraToolRow is one tool row of the map, the native rows aside.
func infraToolRow(sonar, tool, rule string, where ...string) Row {
	return Row{Sonar: sonar, Tool: tool, Rule: rule, Where: where, Match: "close", Set: model.SurfaceParity}
}

// infraToolRows answers the rows whose tool is an infrastructure driver.
func infraToolRows() []Row {
	all := infraRows()
	rows := make([]Row, 0, len(all))
	for _, r := range all {
		if slices.ContainsFunc(infraDrivers(), func(d Driver) bool { return d.Tool == r.Tool }) {
			rows = append(rows, r)
		}
	}
	return rows
}

func TestInfraRowsMapTheDockerAndCloudFormationKeys(t *testing.T) {
	t.Parallel()
	const cfn, docker = "cloudformation:", "docker:"
	want := []Row{
		infraToolRow(cfn+"S6252", "checkov", "CKV_AWS_21", "L"),
		infraToolRow(cfn+"S6258", "checkov", "CKV_AWS_18", "L"),
		infraToolRow(cfn+"S6258", "checkov", "CKV_AWS_91", "L"),
		infraToolRow(cfn+"S6265", "checkov", "CKV_AWS_20", "L"),
		infraToolRow(cfn+"S6265", "checkov", "CKV_AWS_57", "L"),
		infraToolRow(cfn+"S6281", "checkov", "CKV_AWS_53", "L"),
		infraToolRow(cfn+"S6281", "checkov", "CKV_AWS_54", "L"),
		infraToolRow(cfn+"S6281", "checkov", "CKV_AWS_55", "L"),
		infraToolRow(cfn+"S6281", "checkov", "CKV_AWS_56", "L"),
		infraToolRow(docker+"S6471", "hadolint", "DL3002", "H", "L"),
		infraToolRow(docker+"S6500", "hadolint", "DL3015", "H", "L"),
	}
	if got := infraToolRows(); !reflect.DeepEqual(got, want) {
		t.Errorf("tool rows = %+v\nwant %+v", got, want)
	}
	decoded, err := DecodeRows(infraRuleMap)
	if err != nil || !reflect.DeepEqual(decoded, infraRows()) {
		t.Errorf("DecodeRows(rulemap/infra.yml) = %+v, %v", decoded, err)
	}
}

// TestEveryInfraToolRowHasItsProof binds the rows and the manifest both
// ways: a row without an entry, or an entry without a row, is red.
func TestEveryInfraToolRowHasItsProof(t *testing.T) {
	t.Parallel()
	m, err := proof.LoadManifest(infraManifest)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, r := range infraToolRows() {
		id := "lint." + r.Tool + "-" + Code(r.Rule)
		ids[id] = true
		if e, ok := m.Lookup(id); !ok || e.Code != Code(r.Rule) {
			t.Errorf("row %s %s/%s has no proof entry %s", r.Sonar, r.Tool, r.Rule, id)
		}
	}
	for _, name := range m.Names() {
		if !ids[name] {
			t.Errorf("proof entry %s has no rule map row", name)
		}
	}
}

func TestInfraRuleMapRowsHoldTheirProof(t *testing.T) {
	t.Parallel()
	proveManifest(t, infraManifest, infraDrivers())
}

// TestInfraRuleMapRowsHoldUnderBothSets: every row is a parity row, so its
// red tree yields its code under parity and under strict alike.
func TestInfraRuleMapRowsHoldUnderBothSets(t *testing.T) {
	t.Parallel()
	for _, set := range infraSets {
		proveInfraSet(t, infraManifest, set, infraDrivers())
	}
}
