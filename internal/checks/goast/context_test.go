package goast_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/goast"
)

func TestBareContext(t *testing.T) {
	t.Parallel()
	src := "package a\n\nimport \"context\"\n\nfunc f() context.Context {\n\t_ = context.Background()\n\treturn context." +
		strings.ToUpper("todo") + "()\n}\n"
	res := scan(t, goast.BareContext(), map[string]string{
		"internal/a.go":      src,
		"internal/b.go":      "package a\n\nimport \"context\"\n\nfunc g(ctx context.Context) context.Context {\n\treturn context.WithoutCancel(ctx)\n}\n",
		"internal/a_test.go": src,
		"cmd/a.go":           src,
	})
	expectCodes(t, res, "bare-context", "bare-context")
	expectLines(t, res, 6, 7)
	if !strings.HasPrefix(res.Findings[0].Message, "context.Background creates a context outside the composition root") {
		t.Errorf("message = %q", res.Findings[0].Message)
	}
}
