package paths_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/paths"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestGeneratedTrackedProtectsTheMovedAPIOutput(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, path, content string
		want                int
	}{
		{"red: the moved API output at the root", ".gitignore", "api/gen/\n", 1},
		{"red: the moved API output from the api directory", "api/.gitignore", "gen/**\n", 1},
		{"near: a directory that only starts with gen", ".gitignore", "api/generators/\n", 0},
		{"near: a file named like the output", ".gitignore", "api/gen.yaml\n", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := scan(t, paths.GeneratedTracked(), map[string]string{tc.path: tc.content})
			if got.Status.Kind != model.StatusMeasured || len(got.Findings) != tc.want {
				t.Fatalf("result = %+v, want %d findings", got, tc.want)
			}
		})
	}
}
