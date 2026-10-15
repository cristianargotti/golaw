package goast

import (
	"go/ast"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

const codeLocalClock = "local-clock"

// ClockUTC is goast.clock-utc: the clock the wiring in cmd/ hands to the
// contexts is UTC, so a Clock call that receives time.Now itself is a
// finding.
func ClockUTC() model.Check {
	return check{spec: spec("clock-utc"), judge: judgeClockUTC}
}

func judgeClockUTC(t *target, _ *state) ([]model.Finding, error) {
	if !under(t.file.Path, cmdDir) {
		return nil, nil
	}
	file, _, err := t.parsed()
	if err != nil {
		return nil, err
	}
	var out []model.Finding
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 || !strings.HasSuffix(lastSegment(callee(call)), "Clock") {
			return true
		}
		if pkg, name, ok := selector(call.Args[0]); ok && pkg == "time" && name == "Now" {
			message := "the clock handed to the wiring is the local time.Now: wrap it in a helper returning time.Now().UTC()"
			out = append(out, t.finding(codeLocalClock, call.Pos(), message))
		}
		return true
	})
	return out, nil
}
