package shell_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/shell"
	"github.com/cristianargotti/golaw/internal/model"
)

// The prohibited runes, built from their code points so this file carries
// none.
const (
	emDash = string(rune(0x2014))
	enDash = string(rune(0x2013))
)

func TestDashes(t *testing.T) {
	t.Parallel()
	em := judge(t, shell.Dashes(), "git commit -m 'fix: a "+emDash+" b'")
	en := judge(t, shell.Dashes(), "git commit -m 'fix: a "+enDash+" b'")
	if c := codes(em); len(c) != 1 || c[0] != "em-dash" || em.Findings[0].Severity != model.SeverityDeny {
		t.Errorf("em dash: %v", c)
	}
	if c := codes(en); len(c) != 1 || c[0] != "en-dash" {
		t.Errorf("en dash: %v", c)
	}
	for _, clean := range []string{"git commit -m 'fix: a - b'", "echo '" + emDash + "'", "git commit --amend -m 'a " + emDash + "'"} {
		if c := codes(judge(t, shell.Dashes(), clean)); len(c) != 0 {
			t.Errorf("%q: %v", clean, c)
		}
	}
}
