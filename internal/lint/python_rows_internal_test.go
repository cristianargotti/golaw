package lint

import (
	"reflect"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func pythonRuffRow(sonar, rule, match string) Row {
	return Row{Sonar: sonar, Tool: pythonRuff, Rule: rule, Where: []string{"H", "L"}, Match: match, Set: model.SurfaceParity}
}

func pythonExpectRows(t *testing.T, want []Row) {
	t.Helper()
	rows := pythonRows()
	for _, w := range want {
		i := slices.IndexFunc(rows, func(r Row) bool { return r.Sonar == w.Sonar && r.Tool == w.Tool && r.Rule == w.Rule })
		if i < 0 {
			t.Errorf("no row %s %s %s", w.Sonar, w.Tool, w.Rule)
			continue
		}
		if !reflect.DeepEqual(rows[i], w) {
			t.Errorf("row = %+v, want %+v", rows[i], w)
		}
	}
}

func TestPythonRulemapDecodesStrictly(t *testing.T) {
	t.Parallel()
	rows, err := DecodeRows(pythonRulemap)
	if err != nil {
		t.Fatalf("rulemap/python.yml: %v", err)
	}
	if len(rows) == 0 || len(rows) != len(pythonRows()) {
		t.Errorf("decoded %d rows, pythonRows answers %d", len(rows), len(pythonRows()))
	}
}

func TestPythonRowsHoldTheHubClasses(t *testing.T) {
	t.Parallel()
	pythonExpectRows(t, []Row{
		pythonRuffRow("python:S1523", "S102", "exact"),
		pythonRuffRow("python:S1523", "S307", "exact"),
		pythonRuffRow("python:S2068", "S105", "close"),
		pythonRuffRow("python:S2068", "S106", "close"),
		pythonRuffRow("python:S2068", "S107", "close"),
		pythonRuffRow("python:S3776", "C901", "close"),
		pythonRuffRow("python:S3776", "PLR0912", "close"),
		pythonRuffRow("python:S5754", "BLE001", "close"),
		pythonRuffRow("python:S5754", "E722", "close"),
		pythonRuffRow("python:S7519", "C420", "exact"),
		{Sonar: "python:S8495", Tool: pythonPyright, Rule: "reportAssignmentType", Where: []string{"L"}, Match: "close", Set: model.SurfaceParity},
	})
}

func TestPythonRowsLeaveTheGapsUnclaimed(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		sonar   string
		claimed bool
	}{
		"python:S1192 duplicated literal is a gap of M6.4.4":    {sonar: "python:S1192", claimed: false},
		"python:S1313 literal address is a gap of M6.4.4":       {sonar: "python:S1313", claimed: false},
		"python:S5852 regex backtracking is a gap of M6.4.4":    {sonar: "python:S5852", claimed: false},
		"python:S2068 the hub's test passwords (near miss)":     {sonar: "python:S2068", claimed: true},
		"python:S3776 the hub's hygiene complexity (near miss)": {sonar: "python:S3776", claimed: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			claimed := slices.ContainsFunc(pythonRows(), func(r Row) bool { return r.Sonar == tc.sonar })
			if claimed != tc.claimed {
				t.Errorf("a row claims %s = %v, want %v", tc.sonar, claimed, tc.claimed)
			}
		})
	}
}

func TestPythonRowsAreTheParitySelection(t *testing.T) {
	t.Parallel()
	var ruffRules []string
	for _, r := range pythonRows() {
		if r.Set != model.SurfaceParity {
			t.Errorf("row %s %s is in set %q, want parity", r.Sonar, r.Rule, r.Set)
		}
		switch r.Tool {
		case pythonRuff:
			ruffRules = append(ruffRules, r.Rule)
		case pythonPyright:
			if got := pythonReadPyright(t, model.SurfaceParity)[r.Rule]; got != "error" {
				t.Errorf("parity pyright sets %s to %q, want error", r.Rule, got)
			}
		default:
			t.Errorf("row %s names tool %q", r.Sonar, r.Tool)
		}
	}
	slices.Sort(ruffRules)
	ruffRules = slices.Compact(ruffRules)
	if got := pythonReadRuff(t, model.SurfaceParity).Select; !slices.Equal(ruffRules, got) {
		t.Errorf("parity select = %v, the ruff rows name %v", got, ruffRules)
	}
}
