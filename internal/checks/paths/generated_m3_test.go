package paths_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/paths"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestGeneratedM3Trees(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, path, content string
		want                int
	}{
		{"API root red", ".gitignore", "api/generated/\n", 1},
		{"API nested red", "api/.gitignore", "generated/**\n", 1},
		{"API rooted nested red", "api/.gitignore", "/generated/\n", 1},
		{"database red", ".gitignore", "internal/adapters/outbound/postgres/sqlcgen/\n", 1},
		{"mocks red", "test/.gitignore", "mocks/\n", 1},
		{"similar API path near", ".gitignore", "api/generated-docs/\n", 0},
		{"unrelated generated directory near", ".gitignore", "generated/\n", 0},
		{"unrelated nested directory near", "docs/.gitignore", "generated/\n", 0},
		{"comment near", "api/.gitignore", "# generated/\n", 0},
		{"negation near", "api/.gitignore", "!generated/\n", 0},
		{"ordinary output near", ".gitignore", "bin/\ncoverage.out\n", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := scan(t, paths.GeneratedTracked(), map[string]string{tc.path: tc.content})
			if got.Status.Kind != model.StatusMeasured || len(got.Findings) != tc.want {
				t.Fatalf("result = %+v, want %d findings", got, tc.want)
			}
			for _, finding := range got.Findings {
				if finding.Code != "generated-ignored" || finding.Path != tc.path || finding.Line != 1 {
					t.Fatalf("finding = %+v", finding)
				}
			}
		})
	}
}
