package goast_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/goast"
)

func TestWallClock(t *testing.T) {
	t.Parallel()
	src := "package a\n\nimport \"time\"\n\nfunc f(t time.Time) (time.Duration, time.Duration) {\n\t_ = time.Now()\n\treturn time.Since(t), time.Until(t)\n}\n"
	res := scan(t, goast.WallClock(), map[string]string{
		"internal/domain/a.go":      src,
		"internal/application/a.go": src,
		"internal/adapters/a.go":    src,
		"cmd/a.go":                  src,
		"internal/domain/a_test.go": src,
	})
	expectCodes(t, res, "wall-clock", "wall-clock", "wall-clock", "wall-clock", "wall-clock", "wall-clock")
	expectLines(t, res, 6, 7, 7, 6, 7, 7)
	if res.Findings[0].Path != "internal/application/a.go" || res.Findings[3].Path != "internal/domain/a.go" {
		t.Errorf("paths = %+v", res.Findings)
	}
	want := "time.Since reads the wall clock in the domain or a use case: take now as a parameter or a clock function"
	if res.Findings[1].Message != want {
		t.Errorf("message = %q", res.Findings[1].Message)
	}
}
