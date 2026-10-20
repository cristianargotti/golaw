package vuln_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/vuln"
)

// stream is a govulncheck JSON stream with one advisory the code only
// depends on and three it reaches, one of them without a fix and one
// reported by two traces.
const stream = `{"config": {"scanner_name": "govulncheck", "scanner_version": "v1.7.0"}}
{"progress": {"message": "Scanning"}}
{"osv": {"id": "GO-2026-0001"}}
{"finding": {"osv": "GO-2026-0001", "fixed_version": "v1.27.2", "trace": [{"module": "stdlib", "version": "v1.27.0"}]}}
{"finding": {"osv": "GO-2026-0003", "trace": [{"module": "example.test/dep", "package": "example.test/dep", "function": "Do"}]}}
{"finding": {"osv": "GO-2026-0002", "fixed_version": "v1.27.2", "trace": [{"module": "stdlib", "package": "net/http", "function": "Serve"}, {"module": "example.test/m", "function": "main"}]}}
{"finding": {"osv": "GO-2026-0002", "fixed_version": "v1.27.2", "trace": [{"module": "stdlib", "package": "net/http", "function": "ListenAndServe"}]}}
`

func TestReachableKeepsTheAdvisoriesTheCodeReaches(t *testing.T) {
	t.Parallel()
	found, err := vuln.Reachable(strings.NewReader(stream))
	want := []vuln.Finding{
		{ID: "GO-2026-0002", Module: "stdlib", Fixed: "v1.27.2"},
		{ID: "GO-2026-0003", Module: "example.test/dep"},
	}
	if err != nil || !slices.Equal(found, want) {
		t.Errorf("Reachable = %+v, %v, want %+v", found, err, want)
	}
	clean, err := vuln.Reachable(strings.NewReader(`{"config": {"scanner_version": "v1.7.0"}}`))
	if err != nil || len(clean) != 0 {
		t.Errorf("a scan without findings = %+v, %v", clean, err)
	}
}

// An advisory govulncheck reports in two modules keeps one finding per
// module and fix, so the refusal names every fix the person needs, and
// govulncheck orders its findings as it pleases, so every order of one
// stream answers the same findings.
func TestReachableKeepsEveryModuleOfAnAdvisoryInAnyOrder(t *testing.T) {
	t.Parallel()
	reports := []string{
		`{"finding": {"osv": "GO-2026-0002", "fixed_version": "v0.30.0", "trace": [{"module": "golang.org/x/net", "function": "ServeConn"}]}}`,
		`{"finding": {"osv": "GO-2026-0002", "fixed_version": "v1.27.3", "trace": [{"module": "stdlib", "function": "Serve"}]}}`,
		`{"finding": {"osv": "GO-2026-0002", "fixed_version": "v1.27.3", "trace": [{"module": "stdlib", "function": "ListenAndServe"}]}}`,
		`{"finding": {"osv": "GO-2026-0002", "fixed_version": "v0.31.0", "trace": [{"module": "golang.org/x/net"}]}}`,
		`{"finding": {"osv": "GO-2026-0001", "fixed_version": "v1.27.3", "trace": [{"module": "stdlib", "function": "Open"}]}}`,
	}
	want := []vuln.Finding{
		{ID: "GO-2026-0001", Module: "stdlib", Fixed: "v1.27.3"},
		{ID: "GO-2026-0002", Module: "golang.org/x/net", Fixed: "v0.30.0"},
		{ID: "GO-2026-0002", Module: "stdlib", Fixed: "v1.27.3"},
	}
	for _, order := range permutations(len(reports)) {
		lines := []string{`{"config": {"scanner_version": "v1.7.0"}}`}
		for _, i := range order {
			lines = append(lines, reports[i])
		}
		found, err := vuln.Reachable(strings.NewReader(strings.Join(lines, "\n")))
		if err != nil || !slices.Equal(found, want) {
			t.Fatalf("order %v: Reachable = %+v, %v, want %+v", order, found, err, want)
		}
	}
}

// permutations answers every order of the indexes 0 to n-1.
func permutations(n int) [][]int {
	if n == 0 {
		return [][]int{{}}
	}
	var out [][]int
	for _, rest := range permutations(n - 1) {
		for at := 0; at <= len(rest); at++ {
			order := slices.Insert(slices.Clone(rest), at, n-1)
			out = append(out, order)
		}
	}
	return out
}

// A scanner that stops before its config line leaves the pipe empty, a
// stream without that line never came from a scan, and one cut short lost
// what came after the cut; none may read as clean.
func TestReachableRefusesAStreamThatIsNoScan(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"empty stream": "",
		"no config":    `{"finding": {"osv": "GO-1", "trace": [{"module": "m", "function": "F"}]}}`,
		"cut short":    `{"config": {"scanner_version": "v1.7.0"}}` + "\n{\"finding\": {",
	} {
		if _, err := vuln.Reachable(strings.NewReader(input)); !errors.Is(err, vuln.ErrStream) {
			t.Errorf("%s: %v, want ErrStream", name, err)
		}
	}
}

func TestScannerNamesTheVersionThatWroteTheStream(t *testing.T) {
	t.Parallel()
	if got, err := vuln.Scanner(strings.NewReader(stream)); err != nil || got != "v1.7.0" {
		t.Errorf("Scanner = %q, %v", got, err)
	}
	if _, err := vuln.Scanner(strings.NewReader(`{"progress": {}}`)); !errors.Is(err, vuln.ErrStream) {
		t.Errorf("a stream without config: %v", err)
	}
}

func TestJudgeHoldsAWaiverThroughItsLastDay(t *testing.T) {
	t.Parallel()
	found := []vuln.Finding{{ID: "GO-2026-0002", Module: "stdlib", Fixed: "v1.27.2"}, {ID: "GO-2026-0003", Module: "example.test/dep"}}
	waivers := []model.Waiver{
		{ID: "GO-2026-0002", Until: "2026-12-31", Record: "docs/adr/0003-serve.md"},
		{ID: "GO-2026-0009", Until: "2026-01-01", Record: "docs/adr/0004-old.md"},
	}
	lastDay := time.Date(2026, time.December, 31, 23, 59, 0, 0, time.UTC)
	if v := vuln.Judge(found, waivers, lastDay); !slices.Equal(v.Waived, found[:1]) || !slices.Equal(v.Refused, found[1:]) {
		t.Errorf("on the last day = %+v", v)
	}
	dayAfter := time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)
	if v := vuln.Judge(found, waivers, dayAfter); len(v.Waived) != 0 || !slices.Equal(v.Refused, found) {
		t.Errorf("the day after = %+v", v)
	}
	local := time.Date(2026, time.December, 31, 22, 0, 0, 0, time.FixedZone("west", -3*3600))
	if v := vuln.Judge(found, waivers, local); len(v.Waived) != 0 {
		t.Errorf("a waiver reads its day in UTC: %+v", v)
	}
	broken := []model.Waiver{{ID: "GO-2026-0002", Until: "soon", Record: "r.md"}}
	if v := vuln.Judge(found, broken, lastDay); len(v.Waived) != 0 {
		t.Errorf("a day that does not parse holds nothing: %+v", v)
	}
}

func TestExpiredNamesTheWaiversPastTheirLastDay(t *testing.T) {
	t.Parallel()
	waivers := []model.Waiver{
		{ID: "GO-2026-0002", Until: "2026-12-31", Record: "docs/adr/0003-serve.md"},
		{ID: "GO-2026-0009", Until: "2026-01-01", Record: "docs/adr/0004-old.md"},
	}
	got := vuln.Expired(waivers, time.Date(2026, time.October, 19, 0, 0, 0, 0, time.UTC))
	if !slices.Equal(got, waivers[1:]) {
		t.Errorf("Expired = %+v", got)
	}
}
