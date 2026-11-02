package gen_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/kinds"
)

// A use case that names two ports of an area without a ports.go yet: the
// second port must land in the ports.go the first one creates in the same
// run, or the use cases file names a type no file declares.
func TestNewUsecaseWithTwoNewPortsDeclaresBoth(t *testing.T) {
	t.Parallel()
	req := genkind.Request{
		Name:  genkind.Name{Area: "growth", Type: "ReadPlan"},
		Ports: []string{"Ledger", "Clock"}, In: "ReadPlanInput", Out: "Plan",
	}
	files := map[string][]byte{"go.mod": []byte("module example.com/slice\n")}
	res, err := gen.New(t.Context(), engineInput(t, files, empty(), kinds.All()...), genkind.KindUsecase, req)
	if err != nil {
		t.Fatal(err)
	}
	ports := string(writeOf(t, res, "internal/application/growth/ports.go").Content)
	for _, port := range []string{"type Ledger interface", "type Clock interface"} {
		if !strings.Contains(ports, port) {
			t.Errorf("ports.go lacks %q:\n%s", port, ports)
		}
	}
	if strings.Count(ports, "package growth") != 1 {
		t.Errorf("ports.go holds more than one package clause:\n%s", ports)
	}
}
