package goast_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/goast"
)

func TestClockUTC(t *testing.T) {
	t.Parallel()
	red := "package main\n\nimport \"time\"\n\nfunc main() {\n\t_ = Clock(time.Now)\n\t_ = wiring.NewClock(time.Now)\n}\n"
	near := "package main\n\nimport \"time\"\n\nfunc main() {\n\t_ = Clock(utcNow)\n\t_ = Clock(time.Now())\n\t_ = Clock(time.Now, 1)\n\t_ = Other(time.Now)\n}\n"
	res := scan(t, goast.ClockUTC(), map[string]string{
		"cmd/app/main.go":      red,
		"cmd/app/near.go":      near,
		"internal/wiring/a.go": red,
	})
	expectCodes(t, res, "local-clock", "local-clock")
	expectLines(t, res, 6, 7)
	want := "the clock handed to the wiring is the local time.Now: wrap it in a helper returning time.Now().UTC()"
	if res.Findings[0].Path != "cmd/app/main.go" || res.Findings[0].Message != want {
		t.Errorf("finding = %+v", res.Findings[0])
	}
}
