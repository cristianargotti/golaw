package runner_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/runner"
)

func affectedRepo(t *testing.T) *gittest.Repo {
	t.Helper()
	repo := gittest.New(t)
	files := map[string]string{
		"go.mod":        "module example.test/graph\n\ngo 1.24\n",
		"leaf/value.go": "package leaf\nconst Value = 1\n",
		"direct/value.go": `package direct
import "example.test/graph/leaf"
const Value = leaf.Value
`,
		"app/value.go": `package app
import "example.test/graph/direct"
const Value = direct.Value
`,
		"inside/value.go": "package inside\nconst Value = 1\n",
		"inside/value_test.go": `package inside
import (
	"testing"
	"example.test/graph/leaf"
)
func TestValue(t *testing.T) {
	if leaf.Value != Value {
		t.Fatal("value differs")
	}
}
`,
		"outside/value.go": "package outside\nconst Value = 1\n",
		"outside/value_test.go": `package outside_test
import (
	"testing"
	"example.test/graph/leaf"
)
func TestValue(t *testing.T) {
	if leaf.Value != 1 {
		t.Fatal("value differs")
	}
}
`,
		"unrelated/value.go":            "package unrelated\nconst Value = 1\n",
		"leaf/testdata/cases/input.txt": "input\n",
		"docs/readme.md":                "Documentation.\n",
	}
	for name, content := range files {
		repo.Write(name, content)
	}
	return repo
}

// affectedCase is one row of the touched closure: the changed paths, and
// the patterns and the all flag Affected answers for them.
type affectedCase struct {
	name     string
	paths    []string
	patterns []string
	all      bool
}

func TestAffectedSelectsOnlyTheTouchedClosure(t *testing.T) {
	t.Parallel()
	repo := affectedRepo(t)
	closure := []string{"./app", "./direct", "./inside", "./leaf", "./outside"}
	all := append(slices.Clone(closure), "./unrelated")
	for _, tc := range []affectedCase{
		{"red-source", []string{"leaf/value.go"}, closure, false},
		{"red-deleted-file", []string{"leaf/deleted.go"}, closure, false},
		{"red-testdata", []string{"leaf/testdata/cases/input.txt"}, closure, false},
		{"red-package-data", []string{"leaf/config.json"}, closure, false},
		{"red-internal-test", []string{"inside/value_test.go"}, []string{"./inside"}, false},
		{"red-external-test", []string{"outside/value_test.go"}, []string{"./outside"}, false},
		{"red-module", []string{"go.mod"}, []string{"./..."}, true},
		{"red-sums", []string{"go.sum"}, []string{"./..."}, true},
		{"near-document", []string{"docs/readme.md"}, nil, false},
		{"near-tools-module", []string{"tools/go.mod"}, nil, false},
		{"near-sibling", []string{"unrelated/value.go"}, []string{"./unrelated"}, false},
		{"near-empty", nil, nil, false},
		{"near-duplicate", []string{"leaf/value.go", "leaf/value.go"}, closure, false},
		{"near-symlink-root", []string{"leaf/value.go"}, closure, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			checkAffected(t, affectedRoot(t, repo, tc.name), tc, all)
		})
	}
}

// affectedRoot is the root a row reads the repository through: the
// repository itself, or a symlink to it for near-symlink-root.
func affectedRoot(t *testing.T, repo *gittest.Repo, name string) string {
	t.Helper()
	if name != "near-symlink-root" {
		return repo.Root
	}
	root := filepath.Join(t.TempDir(), "repository")
	if err := os.Symlink(repo.Root, root); err != nil {
		t.Skipf("host cannot create a symlink: %v", err)
	}
	return root
}

// checkAffected requires Affected over the paths of a row to answer its
// patterns, and the packages of those patterns, or of every package when
// the change of the row reaches the module.
func checkAffected(t *testing.T, root string, tc affectedCase, all []string) {
	t.Helper()
	env := append(gittest.Isolated(), "GOWORK=off", "GOPROXY=off")
	got, err := runner.Affected(t.Context(), root, tc.paths, env)
	if err != nil {
		t.Fatal(err)
	}
	if got.All != tc.all || !slices.Equal(got.Patterns, tc.patterns) {
		t.Fatalf("affected = %+v", got)
	}
	patterns := tc.patterns
	if tc.all {
		patterns = all
	}
	packages := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		packages = append(packages, "example.test/graph"+pattern[1:])
	}
	if !slices.Equal(got.Packages, packages) {
		t.Fatalf("packages = %v, want %v", got.Packages, packages)
	}
}

func TestAffectedKeepsImportersOfADeletedPackage(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	repo.Write("go.mod", "module example.test/deleted\n\ngo 1.24\n")
	repo.Write("consumer/value.go", `package consumer
import "example.test/deleted/gone"
const Value = gone.Value
`)
	for _, tc := range []struct {
		name     string
		path     string
		patterns []string
		packages []string
	}{
		{
			"red-deleted-package", "gone/value.go",
			[]string{"./consumer"},
			[]string{"example.test/deleted/consumer"},
		},
		{"near-unrelated-path", "docs/readme.md", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			env := append(gittest.Isolated(), "GOWORK=off", "GOPROXY=off")
			got, err := runner.Affected(
				t.Context(), repo.Root, []string{tc.path}, env,
			)
			if err != nil || got.All ||
				!slices.Equal(got.Patterns, tc.patterns) ||
				!slices.Equal(got.Packages, tc.packages) {
				t.Fatalf("affected = %+v, %v", got, err)
			}
		})
	}
}

func TestAffectedIncludesRootTestImporters(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	repo.Write("go.mod", "module example.test/root\n\ngo 1.24\n")
	repo.Write("value.go", "package root\nconst Value = 1\n")
	repo.Write("leaf/value.go", "package leaf\nconst Value = 1\n")
	repo.Write("value_test.go", `package root_test
import (
	"testing"
	"example.test/root/leaf"
)
func TestValue(t *testing.T) {
	if leaf.Value != 1 {
		t.Fatal("value differs")
	}
}

`)
	env := append(gittest.Isolated(), "GOWORK=off", "GOPROXY=off")
	got, err := runner.Affected(t.Context(), repo.Root, []string{"leaf/value.go"}, env)
	if err != nil || got.All || !slices.Equal(got.Patterns, []string{".", "./leaf"}) ||
		!slices.Equal(got.Packages, []string{"example.test/root", "example.test/root/leaf"}) {
		t.Fatalf("root test importers = %+v, %v", got, err)
	}
}

func TestAffectedEmptyModule(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	repo.Write("go.mod", "module example.test/empty\n\ngo 1.24\n")
	for _, tc := range []struct {
		path     string
		all      bool
		patterns []string
	}{
		{"go.mod", true, []string{"./..."}},
		{"go.sum", true, []string{"./..."}},
		{"docs/readme.md", false, nil},
	} {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			env := append(gittest.Isolated(), "GOWORK=off", "GOPROXY=off")
			got, err := runner.Affected(t.Context(), repo.Root, []string{tc.path}, env)
			if err != nil || got.All != tc.all || len(got.Packages) != 0 ||
				!slices.Equal(got.Patterns, tc.patterns) {
				t.Fatalf("empty module = %+v, %v", got, err)
			}
		})
	}
}

func TestAffectedIgnoresUnresolvedSourceImports(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	repo.Write("go.mod", "module example.test/imports\n\ngo 1.24\n")
	repo.Write("broken/a.go", "package broken\nimport _ \"./ok\"\n")
	repo.Write("unclean/a.go", "package unclean\nimport _ \"example.test/imports/leaf/../leaf\"\n")
	repo.Write("leaf/value.go", "package leaf\nconst Value = 1\n")
	repo.Write("direct/value.go", "package direct\nimport \"example.test/imports/leaf\"\nconst Value = leaf.Value\n")
	env := append(gittest.Isolated(), "GOWORK=off", "GOPROXY=off")
	got, err := runner.Affected(t.Context(), repo.Root, []string{"leaf/value.go"}, env)
	if err != nil || got.All || !slices.Equal(got.Patterns, []string{"./direct", "./leaf"}) ||
		!slices.Equal(got.Packages, []string{"example.test/imports/direct", "example.test/imports/leaf"}) {
		t.Fatalf("unresolved source imports = %+v, %v", got, err)
	}
}
