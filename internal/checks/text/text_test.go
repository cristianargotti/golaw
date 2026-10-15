package text_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/text"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// The prohibited runes, built from their code points so this file carries
// none.
const (
	emDash = string(rune(0x2014))
	enDash = string(rune(0x2013))
)

// run dispatches one check over an in-memory tree in scan mode under the
// proof policy and returns its result.
func run(t *testing.T, c model.Check, files map[string]string) model.Result {
	t.Helper()
	content := make(map[string][]byte, len(files))
	for path, body := range files {
		content[path] = []byte(body)
	}
	tree := proof.NewTree(content)
	res, err := c.Run(t.Context(), model.Input{Mode: model.ModeScan, Policy: proof.Policy(), Snapshot: tree, Targets: tree.Files()})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// codes lists the finding codes of a result in order.
func codes(res model.Result) []string {
	out := make([]string, 0, len(res.Findings))
	for _, f := range res.Findings {
		out = append(out, f.Code)
	}
	return out
}

// broken is a snapshot whose every read fails, to prove Unmeasured.
type broken struct{}

func (broken) Files() []model.File { return []model.File{{Path: "a.go", Class: model.ClassTracked}} }

func (broken) Read(string) ([]byte, error) { return nil, model.ErrUnreadable }

func (broken) Class(string) model.FileClass { return model.ClassTracked }

func TestChecksAreValidAndProofKeyed(t *testing.T) {
	t.Parallel()
	for _, c := range text.Checks() {
		s := c.Spec()
		if err := s.Validate(); err != nil {
			t.Errorf("%s: %v", s.ID, err)
		}
		if s.Proof != s.ID || s.Scope != model.ScopeFile || s.Owner != model.OwnerCore {
			t.Errorf("%s: spec = %+v", s.ID, s)
		}
	}
}

func TestDashes(t *testing.T) {
	t.Parallel()
	res := run(t, text.Dashes(), map[string]string{
		"docs/a.md": "title " + emDash + " subtitle\nrange 1" + enDash + "2\n",
		"b.go":      "package b // a - b\n",
		"c.png":     "\x00binary " + emDash,
	})
	if res.Status.Kind != model.StatusMeasured || res.Scope != "2 files" {
		t.Errorf("status = %+v, scope %q", res.Status, res.Scope)
	}
	if got := codes(res); !slices.Equal(got, []string{"em-dash", "en-dash"}) {
		t.Errorf("codes = %v", got)
	}
	first := res.Findings[0]
	if first.Path != "docs/a.md" || first.Line != 1 || first.Column != 7 || first.Severity != model.SeverityDeny {
		t.Errorf("finding = %+v", first)
	}
	if res.Findings[1].Line != 2 || res.Findings[1].Column != 8 {
		t.Errorf("second finding = %+v", res.Findings[1])
	}
}

func TestUnreadableTargetIsUnmeasured(t *testing.T) {
	t.Parallel()
	tree := broken{}
	res, err := text.Dashes().Run(t.Context(), model.Input{Mode: model.ModeScan, Policy: proof.Policy(), Snapshot: tree, Targets: tree.Files()})
	if err != nil || res.Status.Kind != model.StatusUnmeasured {
		t.Errorf("result = %+v, %v", res, err)
	}
	if res.Status.Reason != "read a.go: path unreadable" {
		t.Errorf("reason = %q", res.Status.Reason)
	}
}
