package text_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/text"
)

func TestDebugPrint(t *testing.T) {
	t.Parallel()
	res := run(t, text.DebugPrint(), map[string]string{
		"internal/a.go":      "fmt.Println(x)\nfmt.Fprintln(w, x)\nfmt.Printf(\"%d\", x)\n",
		"cmd/golaw/main.go":  "fmt.Println(x)\n",
		"scripts/gen.go":     "fmt.Print(x)\n",
		"internal/a_test.go": "fmt.Println(x)\n",
	})
	if got := codes(res); !slices.Equal(got, []string{"debug-print", "debug-print"}) {
		t.Errorf("codes = %v", got)
	}
	if res.Findings[1].Line != 3 || res.Scope != "1 files" {
		t.Errorf("finding = %+v, scope %q", res.Findings[1], res.Scope)
	}
}
