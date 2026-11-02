package imports_test

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/imports"
)

// TestApplyKeepsTheFileBuildableInTheShapesGofmtLeaves pins the import
// layouts a line edit could break: a block that goes whole while a path is
// added, a block on one line, a single spec declaration and an empty block.
func TestApplyKeepsTheFileBuildableInTheShapesGofmtLeaves(t *testing.T) {
	t.Parallel()
	tests := map[string]struct{ in, want string }{
		"red: the only block goes and a path comes": {
			in:   "package x\n\nimport (\n\t\"strings\"\n)\n\nfunc f() { fmt.Println() }\n",
			want: "package x\n\nimport (\n\t\"fmt\"\n)\n\nfunc f() { fmt.Println() }\n",
		},
		"red: a block on one line gets a block of its own": {
			in:   "package x\n\nimport (\"fmt\")\n\nfunc f() { fmt.Println(strings.ToUpper(\"a\")) }\n",
			want: "package x\n\nimport (\n\t\"fmt\"\n)\nimport (\n\t\"strings\"\n)\n\nfunc f() { fmt.Println(strings.ToUpper(\"a\")) }\n",
		},
		"red: a single spec declaration keeps its line": {
			in:   "package x\n\nimport \"fmt\"\n\nfunc f() { fmt.Println(growth.Plan{}) }\n",
			want: "package x\n\nimport \"fmt\"\nimport (\n\t\"example.com/slice/internal/application/growth\"\n)\n\nfunc f() { fmt.Println(growth.Plan{}) }\n",
		},
		"red: an empty block gets both groups": {
			in:   "package x\n\nimport (\n)\n\nfunc f() { fmt.Println(growth.Plan{}) }\n",
			want: "package x\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/slice/internal/application/growth\"\n)\n\nfunc f() { fmt.Println(growth.Plan{}) }\n",
		},
		"near: a file without a final newline": {
			in:   "package x\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\nvar _ = fmt.Sprint",
			want: "package x\n\nimport (\n\t\"fmt\"\n)\n\nvar _ = fmt.Sprint\n",
		},
		"near: names a range and a label declare keep their imports": {
			in:   "package x\n\nimport (\n\t\"bytes\"\n\t\"strings\"\n)\n\nfunc f(m map[int]int) {\nbytes:\n\tfor strings := range m {\n\t\t_ = strings\n\t\tbreak bytes\n\t}\n}\n",
			want: "package x\n\nimport (\n\t\"bytes\"\n\t\"strings\"\n)\n\nfunc f(m map[int]int) {\nbytes:\n\tfor strings := range m {\n\t\t_ = strings\n\t\tbreak bytes\n\t}\n}\n",
		},
	}
	x := moduleIndex()
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fix, err := imports.Apply("x.go", []byte(tc.in), x)
			if err != nil {
				t.Fatal(err)
			}
			if string(fix.Content) != tc.want {
				t.Errorf("Content =\n%s\nwant\n%s", fix.Content, tc.want)
			}
			if _, err := parser.ParseFile(token.NewFileSet(), "x.go", fix.Content, 0); err != nil {
				t.Errorf("the fix left a file that does not parse: %v", err)
			}
		})
	}
}
