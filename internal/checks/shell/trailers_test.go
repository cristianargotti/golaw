package shell_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/shell"
)

func TestTrailers(t *testing.T) {
	t.Parallel()
	phrase := "Co-" + "authored-by: Someone <someone@example.com>"
	denies(t, shell.Trailers(), "trailer", map[string]bool{
		`git commit -m "fix: x" -m "` + phrase + `"`:           true,
		"git commit -m 'fix: x' --signoff":                     true,
		"git commit -s -m 'fix: x'":                            true,
		"git commit -am 'fix: x'":                              false,
		"git commit -S -m 'fix: x'":                            false,
		"git commit --trailer 'Reviewed-by: x' -m 'fix'":       true,
		"git commit --amend --signoff":                         false,
		"git commit -m 'fix: x'":                               false,
		"git commit --message='fix: gener" + "ated with care'": true,
	})
}
