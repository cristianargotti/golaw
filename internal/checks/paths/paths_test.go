package paths_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/paths"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// scan dispatches one file check over an in-memory tree in scan mode under
// the proof policy.
func scan(t *testing.T, c model.Check, files map[string]string) model.Result {
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

// pathsOf lists the finding paths and codes of a result.
func pathsOf(res model.Result) []string {
	out := make([]string, 0, len(res.Findings))
	for k := range res.Findings {
		f := &res.Findings[k]
		out = append(out, f.Path+" "+f.Code)
	}
	return out
}

func TestChecksAreValidAndProofKeyed(t *testing.T) {
	t.Parallel()
	for _, c := range paths.Checks() {
		s := c.Spec()
		if err := s.Validate(); err != nil {
			t.Errorf("%s: %v", s.ID, err)
		}
		if s.Proof != s.ID || s.Owner != model.OwnerCore {
			t.Errorf("%s: spec = %+v", s.ID, s)
		}
	}
}

func TestForbidden(t *testing.T) {
	t.Parallel()
	res := scan(t, paths.Forbidden(), map[string]string{
		"docs/diagram.png":        "not an image",
		"public/logo.png":         "x",
		"manual/guide.pdf":        "x",
		"testdata/a.png":          "x",
		"src/__fixtures__/a.jpg":  "x",
		"a/fixtures/b.gif":        "x",
		"report.PDF":              "x",
		"deploy.pem":              "x",
		"internal/credentials.go": "package internal\n",
		".env.example":            "KEY=\n",
		"README.md":               "# title\n",
	})
	want := []string{"deploy.pem secret-file", "docs/diagram.png binary-document", "report.PDF binary-document"}
	if got := pathsOf(res); !slices.Equal(got, want) {
		t.Errorf("findings = %v, want %v", got, want)
	}
	if res.Status.Kind != model.StatusMeasured || res.Scope != "11 paths" || res.Findings[0].Severity != model.SeverityDeny {
		t.Errorf("status = %+v, scope %q", res.Status, res.Scope)
	}
}
