package text_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/text"
)

func TestBareNolint(t *testing.T) {
	t.Parallel()
	res := run(t, text.BareNolint(), map[string]string{
		"a.go":      "x //nol" + "int\ny //nol" + "int:gosec\nz //nol" + "int:gosec // why\n",
		"a_test.go": "x // nol" + "int \n",
	})
	if got := codes(res); !slices.Equal(got, []string{"bare-nolint", "bare-nolint", "bare-nolint"}) {
		t.Errorf("codes = %v", got)
	}
	if res.Findings[2].Path != "a_test.go" || res.Findings[2].Column != 3 {
		t.Errorf("finding = %+v", res.Findings[2])
	}
}

func TestBareNolintNeedsTheLinterAndTheReason(t *testing.T) {
	t.Parallel()
	res := run(t, text.BareNolint(), map[string]string{
		"a.go": "a //nol" + "int // the reason\n" +
			"b //nol" + "int:gosec //\n" +
			"c //nol" + "int:gosec //  \n" +
			"d //nol" + "int every linter\n" +
			"e //nol" + "int:gosec,errcheck // why\n" +
			"f //nol" + "int:gosec// why\n" +
			"// Nol" + "int directives are rare\n",
	})
	lines := make([]int, 0, len(res.Findings))
	for _, f := range res.Findings {
		lines = append(lines, f.Line)
	}
	if !slices.Equal(lines, []int{1, 2, 3, 4}) {
		t.Errorf("lines = %v", lines)
	}
}
