package rung_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/rung"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

// One advisory reachable in the standard library and in a dependency, as
// govulncheck reports it in either order.
const (
	netStdlib = `{"finding": {"osv": "GO-2026-0004", "fixed_version": "v1.27.2", "trace": [{"module": "stdlib", "package": "net/http", "function": "Serve"}]}}`
	netModule = `{"finding": {"osv": "GO-2026-0004", "fixed_version": "v0.60.0", "trace": [{"module": "golang.org/x/net", "package": "golang.org/x/net/http2", "function": "ServeConn"}]}}`
)

// An advisory reachable in two modules needs both fixes: the refusal names
// each one, in one order whatever order govulncheck reported them in, and
// a waiver of the advisory is named once.
//
//nolint:paralleltest // the fake tools are found through PATH
func TestPrePushNamesEveryFixOfAnAdvisoryInOneOrder(t *testing.T) {
	r := armedRepo(t, armedPolicy)
	short := "golaw pre-push: " + r.Head()[:12] + ": "
	want := []string{
		short + "GO-2026-0004 reachable in golang.org/x/net, fixed in v0.60.0",
		short + "GO-2026-0004 reachable in stdlib, fixed in v1.27.2",
	}
	for name, order := range map[string][]string{"stdlib first": {netStdlib, netModule}, "module first": {netModule, netStdlib}} {
		fakes(t, map[string]runnertest.Behavior{"govulncheck": scanned("v1.7.0", order...)})
		out := rung.RunPrePush(t.Context(), pushing(t, r, update(r.Head())), "origin", "")
		if out.Code != 1 || len(out.Lines) < len(want) || !slices.Equal(out.Lines[:len(want)], want) {
			t.Errorf("%s = %+v", name, out)
		}
	}
	waived := armedRepo(t, armedPolicy+"vulnerability_waivers:\n  - {id: GO-2026-0004, until: 2026-12-31, record: docs/adr/0005-net.md}\n")
	fakes(t, map[string]runnertest.Behavior{"govulncheck": scanned("v1.7.0", netStdlib, netModule)})
	out := rung.RunPrePush(t.Context(), pushing(t, waived, update(waived.Head())), "origin", "")
	if out.Code != 0 || len(out.Lines) != 1 || !contains(out, "GO-2026-0004 waived until 2026-12-31 (docs/adr/0005-net.md)") {
		t.Errorf("a waived advisory of two modules = %+v", out)
	}
}
