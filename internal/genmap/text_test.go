package genmap_test

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/genmap"
)

// hubSized is a report the size of the pilot repository: forty ports and
// a hundred and twenty use cases in one area, one open hole.
func hubSized() genmap.Report {
	r := genmap.Report{
		Layers: []string{"port internal/application/{area}", "usecase internal/application/{area}"},
		Holes:  []holes.Hole{{ID: "plans.read-plan", Constraint: "call p.queries.ReadPlan", Path: "internal/adapters/out/postgres/plans.go", Line: 41, Func: "(*Plans).ReadPlan"}},
	}
	for i := range 40 {
		r.Ports = append(r.Ports, genmap.Item{Name: fmt.Sprintf("growth.Port%02d", i), Path: "internal/application/growth/ports.go", Details: []string{"methods ReadPlan, ArchivePlan, ListPlans"}})
	}
	for i := range 120 {
		r.Usecases = append(r.Usecases, genmap.Item{Name: fmt.Sprintf("growth.UseCase%03d", i), Path: fmt.Sprintf("internal/application/growth/use_case_%03d.go", i), Details: []string{"ports Ledger, Clock", fmt.Sprintf("in Input%03d", i), "out Plan"}})
	}
	r.Contracts = []string{"api/openapi.yaml", "internal/application/growth/ports.go"}
	return r
}

func TestTextPutsTheHolesFirstAndKeepsTheBudget(t *testing.T) {
	t.Parallel()
	text := hubSized().Text(genmap.DefaultBudget)
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if lines[0] != "golaw map layout: port internal/application/{area}, usecase internal/application/{area}" {
		t.Errorf("first line = %q", lines[0])
	}
	if lines[1] != "hole open internal/adapters/out/postgres/plans.go:41 plans.read-plan call p.queries.ReadPlan" {
		t.Errorf("second line = %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "port growth.Port00 internal/application/growth/ports.go: methods ReadPlan, ArchivePlan, ListPlans") {
		t.Errorf("third line = %q", lines[2])
	}
	if len(text) > genmap.DefaultBudget*4 {
		t.Errorf("text has %d bytes, over the %d of the budget", len(text), genmap.DefaultBudget*4)
	}
	if !regexp.MustCompile(`(?m)^and \d+ more use cases: golaw map --area growth$`).MatchString(text) {
		t.Errorf("no count of the omitted use cases in\n%s", text)
	}
	if whole := hubSized().Text(1 << 20); strings.Contains(whole, " more ") {
		t.Errorf("a budget that fits everything still omits: %q", whole)
	}
}

func TestTextNamesTheWholeReportWhenTheOmittedSpanAreas(t *testing.T) {
	t.Parallel()
	r := genmap.Report{Layers: []string{"port internal/application/{area}"}}
	for i := range 30 {
		r.Ports = append(r.Ports, genmap.Item{Name: fmt.Sprintf("area%02d.Port", i), Path: fmt.Sprintf("internal/application/area%02d/ports.go", i)})
	}
	if text := r.Text(200); !strings.Contains(text, " more ports: golaw map --json\n") {
		t.Errorf("text =\n%s", text)
	}
}

func TestJSONCarriesEverySection(t *testing.T) {
	t.Parallel()
	data, err := hubSized().JSON()
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"layers", "holes", "ports", "usecases", "adapters", "handlers", "contracts"} {
		if _, ok := doc[key]; !ok {
			t.Errorf("JSON lacks %s", key)
		}
	}
	var holesDoc []map[string]any
	if err := json.Unmarshal(doc["holes"], &holesDoc); err != nil || len(holesDoc) != 1 || holesDoc[0]["id"] != "plans.read-plan" {
		t.Errorf("holes = %s, %v", doc["holes"], err)
	}
}
