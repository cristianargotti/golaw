package policyfiles

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestDependencyModulesFollowsActiveMainGraphs(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{
			name: "indirect replacement supplies direct children",
			files: map[string]string{
				"go.mod": "module example.test/app\nrequire example.test/a v1.0.0 // indirect\n" +
					"replace example.test/a => ./local\n",
				"local/go.mod": "module example.test/a\nrequire example.test/child v1.0.0\n",
			},
			want: []string{"go.mod", "local/go.mod"},
		},
		{
			name: "root replacements govern cycles and dependency directives stay inactive",
			files: map[string]string{
				"go.mod": "module example.test/app\nrequire example.test/a v1.0.0\n" +
					"replace example.test/a => ./local\nreplace example.test/b => ./child\n",
				"local/go.mod": "module example.test/a\nrequire example.test/b v1.0.0\n" +
					"replace example.test/b => ../outside\n",
				"child/go.mod": "module example.test/b\nrequire example.test/a v1.0.0\n",
			},
			want: []string{"child/go.mod", "go.mod", "local/go.mod"},
		},
		{
			name: "main and tools govern a shared module separately",
			files: map[string]string{
				"go.mod": "module example.test/app\nrequire example.test/a v1.0.0\n" +
					"replace example.test/a => ./shared\nreplace example.test/b => ./first\n",
				"tools/go.mod": "module example.test/tools\nrequire example.test/a v1.0.0\n" +
					"replace example.test/a => ../shared\nreplace example.test/b => ../second\n",
				"shared/go.mod": "module example.test/a\nrequire example.test/b v1.0.0\n",
				"first/go.mod":  "module example.test/b\n",
				"second/go.mod": "module example.test/b\n",
			},
			want: []string{"first/go.mod", "go.mod", "second/go.mod", "shared/go.mod", "tools/go.mod"},
		},
		{
			name: "unused directives and versioned replacements do not supply a local graph",
			files: map[string]string{
				"go.mod": "module example.test/app\nrequire example.test/a v1.0.0\n" +
					"replace example.test/a => example.test/fork v1.1.0\n" +
					"replace example.test/unused => ../outside\nreplace example.test/other => ./absent\n",
			},
			want: []string{"go.mod"},
		},
		{
			name: "matching version wins over wildcard",
			files: map[string]string{
				"go.mod": "module example.test/app\nrequire example.test/a v1.0.0\n" +
					"replace example.test/a => ./absent\nreplace example.test/a v1.0.0 => ./specific\n",
				"specific/go.mod": "module example.test/a\n",
			},
			want: []string{"go.mod", "specific/go.mod"},
		},
		{
			name: "versioned matching replacement prevents wildcard traversal",
			files: map[string]string{
				"go.mod": "module example.test/app\nrequire example.test/a v1.0.0\n" +
					"replace example.test/a => ./absent\n" +
					"replace example.test/a v1.0.0 => example.test/fork v1.1.0\n",
			},
			want: []string{"go.mod"},
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			snapshot := graphSnapshot(row.files)
			units, err := dependencyModules(model.Input{Snapshot: snapshot})
			if err != nil {
				t.Fatal(err)
			}
			files := make([]string, 0, len(units))
			for _, unit := range units {
				files = append(files, unit.file)
			}
			slices.Sort(files)
			if !slices.Equal(files, row.want) || len(snapshot.reads) != len(files) {
				t.Fatalf("modules = %v, reads = %v, want %v read once", files, snapshot.reads, row.want)
			}
		})
	}
}

func TestReplacementFileNormalizesAndRejectsNativeEscapes(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		file, target, want string
	}{
		{"go.mod", "./local/child/..", "local/go.mod"},
		{"tools/go.mod", "../local", "local/go.mod"},
		{"go.mod", `.\local\child\..`, "local/go.mod"},
		{"go.mod", "../outside", ""},
		{"tools/go.mod", "../../outside", ""},
		{"go.mod", "/outside", ""},
		{"go.mod", `C:\outside`, ""},
		{"go.mod", "C:outside", ""},
		{"go.mod", "./C:/outside", ""},
		{"tools/go.mod", "../C:/outside", ""},
		{"go.mod", `\\server\outside`, ""},
		{"go.mod", "//server/outside", ""},
	} {
		t.Run(row.file+row.target, func(t *testing.T) {
			t.Parallel()
			replacement := moduleReplacement{oldPath: "example.test/a", oldVersion: "v1.0.0", newPath: row.target, line: 7}
			got, err := replacementFile(row.file, replacement)
			if got != row.want {
				t.Fatalf("path = %q, want %q", got, row.want)
			}
			if row.want != "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if !errors.Is(err, model.ErrOutsideSubset) || !strings.Contains(err.Error(), row.file+":7:") ||
				!strings.Contains(err.Error(), "replace example.test/a v1.0.0 => "+row.target) {
				t.Fatalf("unsafe replacement error = %v", err)
			}
		})
	}
}

func TestDependencyModulesPreservesUnavailableGraphEvidence(t *testing.T) {
	t.Parallel()
	const main = "module example.test/app\nrequire example.test/a v1.0.0\nreplace example.test/a => ./local\n"
	for _, row := range []struct {
		name, failed string
		files        map[string]string
		blocked      error
		want         error
	}{
		{"missing root", "go.mod", nil, nil, model.ErrOutsideSubset},
		{"missing local", "local/go.mod", map[string]string{"go.mod": main}, nil, model.ErrOutsideSubset},
		{"unreadable root", "go.mod", map[string]string{"go.mod": main}, model.ErrUnreadable, model.ErrUnreadable},
		{"unreadable local", "local/go.mod", map[string]string{"go.mod": main, "local/go.mod": "module example.test/a\n"}, model.ErrUnreadable, model.ErrUnreadable},
		{"oversized local", "local/go.mod", map[string]string{"go.mod": main, "local/go.mod": "module example.test/a\n"}, model.ErrTooLarge, model.ErrTooLarge},
		{"oversized tools", "tools/go.mod", map[string]string{"go.mod": "module example.test/app\n", "tools/go.mod": "module example.test/tools\n"}, model.ErrTooLarge, model.ErrTooLarge},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			snapshot := graphSnapshot(row.files)
			snapshot.failed, snapshot.failure = row.failed, row.blocked
			units, err := dependencyModules(model.Input{Snapshot: snapshot})
			failed, ok := errors.AsType[*dependencyReadError](err)
			if !errors.Is(err, row.want) || len(units) != 0 {
				t.Fatalf("graph = %+v, error = %v, want %s: %v", units, err, row.failed, row.want)
			}
			if ok != (row.blocked != nil) {
				t.Fatalf("snapshot read wrapper = %v, want it only for a read failure: %v", ok, err)
			}
			if ok && (failed.file != row.failed || err.Error() != row.failed+": "+row.blocked.Error()) {
				t.Fatalf("snapshot error = %v, want the original text and path %s", err, row.failed)
			}
		})
	}
}

func TestDependencyModulesSkipsFixturesBeforeReading(t *testing.T) {
	t.Parallel()
	snapshot := graphSnapshot(map[string]string{
		"go.mod": "module example.test/app\nrequire example.test/a v1.0.0\n" +
			"replace example.test/a => ./testdata/a\n",
		"testdata/a/go.mod":      "invalid module content",
		"testdata/go.work":       "invalid workspace content",
		"testdata/vendor/a/a.go": "package a\n",
	})
	units, err := dependencyModules(model.Input{Snapshot: snapshot})
	if err != nil || len(units) != 1 || !slices.Equal(snapshot.reads, []string{"go.mod"}) {
		t.Fatalf("fixture graph = %+v, error = %v, reads = %v", units, err, snapshot.reads)
	}
}

func TestDependencyModulesKeepsGeneratedModulesAndParseErrors(t *testing.T) {
	t.Parallel()
	for _, content := range []string{"// Code generated by example. DO NOT EDIT.\nmodule example.test/a\n", "invalid module content"} {
		t.Run(content, func(t *testing.T) {
			t.Parallel()
			snapshot := graphSnapshot(map[string]string{"go.mod": "module example.test/app\n", "local/go.mod": content})
			if _, err := dependencyModules(model.Input{Snapshot: snapshot}); !errors.Is(err, model.ErrOutsideSubset) {
				t.Fatalf("unexplained module error = %v", err)
			}
		})
	}
	snapshot := graphSnapshot(map[string]string{"go.mod": "invalid module content"})
	if _, err := dependencyModules(model.Input{Snapshot: snapshot}); err == nil || !strings.Contains(err.Error(), "go.mod") {
		t.Fatalf("parse error = %v", err)
	}
}

type dependencySnapshot struct {
	*proof.Tree

	reads   []string
	failed  string
	failure error
}

func graphSnapshot(files map[string]string) *dependencySnapshot {
	content := make(map[string][]byte, len(files))
	for name, data := range files {
		content[name] = []byte(data)
	}
	return &dependencySnapshot{Tree: proof.NewTree(content)}
}

func (s *dependencySnapshot) Read(file string) ([]byte, error) {
	s.reads = append(s.reads, file)
	if file == s.failed && s.failure != nil {
		return nil, fmt.Errorf("%s: %w", file, s.failure)
	}
	data, err := s.Tree.Read(file)
	if err != nil {
		return nil, fmt.Errorf("read dependency snapshot: %w", err)
	}
	return data, nil
}

func TestDependencyRecordsVendorDirectories(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		path, content string
		limited       bool
	}{
		{"web/static/vendor/app.js", "export const answer = 1;\n", false},
		{"assets/vendor/go.mod", "not a Go module\n", false},
		{"assets/vendor/go.work", "not a Go workspace\n", false},
		{"tools/vendor/modules.txt", "# example.test/a v1.0.0\n", true},
	} {
		t.Run(row.path, func(t *testing.T) {
			t.Parallel()
			near, err := proof.LoadFixture("../../proof/testdata/policy.dependency-records/near")
			if err != nil {
				t.Fatal(err)
			}
			near.Files[row.path] = []byte(row.content)
			got, err := DependencyRecords().Run(t.Context(), model.Input{Snapshot: proof.NewTree(near.Files), Policy: proof.Policy()})
			wantStatus := model.StatusMeasured
			if row.limited {
				wantStatus = model.StatusUnmeasured
			}
			if err != nil || got.Status.Kind != wantStatus || len(got.Findings) != 0 || model.IsOutsideSubset(got.Status) != row.limited {
				t.Fatalf("result = %+v, error %v, want %s", got, err, wantStatus)
			}
			if row.limited && !strings.Contains(got.Status.Reason, "tools/vendor/") {
				t.Fatalf("vendor limit = %q, want the tools vendor directory", got.Status.Reason)
			}
		})
	}
}
