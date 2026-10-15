package shell_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/shell"
)

func TestBodyWidth(t *testing.T) {
	t.Parallel()
	long, short := strings.Repeat("a", 101), strings.Repeat("a", 100)
	denies(t, shell.BodyWidth(), "body-width", map[string]bool{
		`git commit -m "fix: x" -m "` + long + `"`:                     true,
		`git commit -m "fix: x" -m "` + short + `"`:                    false,
		`git commit -m "` + long + `"`:                                 false,
		`git commit --message="fix: x` + "\n" + long + `"`:             true,
		`git commit -m "fix: x" -m "ok" -m "` + long + `"`:             true,
		`git commit -m "fix: x" -m "` + strings.Repeat("é", 100) + `"`: false,
		"git log -n 1": false,
	})
}
