package cicheck_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestOwnershipReadersAgreeOnSupportedPatterns(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, owners, path string
	}{
		{"rooted directory", "* @app\n/docs/ @other\n", "docs/a.txt"},
		{"nested directory", "* @app\ndocs/ @other\n", "a/docs/b.txt"},
		{"last match", "* @app\n/docs/ @other\n/docs/a.txt @app\n", "docs/a.txt"},
		{"owner order", "* @a @b\n/docs/ @B @A\n", "docs/a.txt"},
		{"empty owners", "* @app\n/docs/\n", "docs/a.txt"},
		{"root star", "/* @app\n/docs/ @other\n", "docs/a.txt"},
		{"recursive catchall", "/** @app\n/docs/ @other\n", "docs/a.txt"},
		{"recursive path", "* @app\n/docs/** @other\n", "docs/a/b.txt"},
		{"final bare star", "* @app\n/docs/* @other\n", "docs/a/b.txt"},
		{"suffix", "* @app\n*.md @other\n", "docs/a.md"},
		{"email", "* app@example.test\n/docs/ other@example.test\n", "docs/a.txt"},
		{"dot is literal", "* @app\n.Dockerfile @other\n", "Dockerfile"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			adoption, err := bootstrap.AllowedPaths([]string{tc.path}, []byte(tc.owners))
			if err != nil {
				t.Fatal(err)
			}
			tree := proof.NewTree(map[string][]byte{
				"CODEOWNERS": []byte(tc.owners), tc.path: []byte("content\n"),
			})
			policy := proof.Policy()
			policy.Profile = "fixture"
			result, err := policyfiles.CodeownersForeign().Run(t.Context(), model.Input{
				Mode: model.ModeStaged, Policy: policy, Snapshot: tree,
				Changes: []model.Change{{Op: model.ChangeAdd, NewPath: tc.path, Mode: "100644"}},
			})
			if err != nil || result.Status.Kind != model.StatusMeasured ||
				(len(result.Findings) != 0) != (len(adoption.Protected) != 0) {
				t.Fatalf("adoption = %+v, staged = %+v, error %v", adoption, result, err)
			}
		})
	}
}
