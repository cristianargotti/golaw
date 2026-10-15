package text_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/text"
)

// The markers are assembled from halves so this file carries none.
func TestTodo(t *testing.T) {
	t.Parallel()
	res := run(t, text.TaskMarker(), map[string]string{
		"a.go": "// T" + "ODO: later\n// a todo list\n// FIX" + "ME now\n",
		"a.md": "T" + "ODO in prose is not code\n",
	})
	if got := codes(res); !slices.Equal(got, []string{"todo-marker", "todo-marker"}) {
		t.Errorf("codes = %v", got)
	}
	if res.Findings[1].Line != 3 || res.Findings[1].Column != 4 {
		t.Errorf("finding = %+v", res.Findings[1])
	}
}
