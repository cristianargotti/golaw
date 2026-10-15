package text_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/text"
)

func TestNolintNilnil(t *testing.T) {
	t.Parallel()
	line := "return nil, nil //nol" + "int:wrapcheck,nilnil // legacy\n"
	res := run(t, text.NolintNilnil(), map[string]string{"a.go": line, "a_test.go": line})
	if got := codes(res); !slices.Equal(got, []string{"nolint-nilnil"}) {
		t.Errorf("codes = %v", got)
	}
	if res.Findings[0].Path != "a.go" || res.Findings[0].Column != 17 {
		t.Errorf("finding = %+v", res.Findings[0])
	}
}
