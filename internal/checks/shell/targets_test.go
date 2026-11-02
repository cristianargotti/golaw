package shell_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/shell"
)

func TestWriteTargetsAreTheLiteralFilesACommandWrites(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		command string
		want    []string
	}{
		"red: a here-document redirection":            {"cat > internal/x/zz_generated_a.go <<'EOF'\npackage x\nEOF\n", []string{"internal/x/zz_generated_a.go"}},
		"red: an append":                              {"echo hi >> a.go", []string{"a.go"}},
		"red: a stream named before the operator":     {"gen 2> errors.log", []string{"errors.log"}},
		"red: tee with an option":                     {"printf x | tee -a a.go b.go", []string{"a.go", "b.go"}},
		"red: sed in place":                           {"sed -i 's/a/b/' a.go", []string{"a.go"}},
		"red: sed in place with an expression option": {"sed -i.bak -e 's/a/b/' a.go b.go", []string{"a.go", "b.go"}},
		"red: cp names its destination":               {"cp src.go dst.go", []string{"dst.go"}},
		"red: mv names both ends":                     {"mv old.go new.go", []string{"new.go", "old.go"}},
		"red: rm names every operand":                 {"rm -f a.go b.go", []string{"a.go", "b.go"}},
		"red: a shell that runs a script":             {"bash -lc 'echo x > a.go'", []string{"a.go"}},
		"red: an absolute path":                       {"echo x > /project/a.go", []string{"/project/a.go"}},
		"near: a variable target":                     {"echo x > \"$OUT\"", nil},
		"near: sed without in place":                  {"sed 's/a/b/' a.go", nil},
		"near: cp with a target directory option":     {"cp -t dir a.go", nil},
		"near: a stream duplication":                  {"echo x 2>&1", nil},
		"near: a glob":                                {"rm *.go", nil},
		"near: an input redirection":                  {"cat < a.go", nil},
		"near: a command substitution":                {"echo x > $(pick)", nil},
		"near: a home path":                           {"echo x > ~/a.go", nil},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := shell.WriteTargets(tc.command, "Bash"); !slices.Equal(got, tc.want) {
				t.Errorf("WriteTargets = %q, want %q", got, tc.want)
			}
		})
	}
}
