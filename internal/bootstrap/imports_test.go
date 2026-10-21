package bootstrap_test

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/model"
)

func importsModule() bootstrap.ModuleInfo {
	return bootstrap.ModuleInfo{
		Path: "example.test/birth", GoVersion: "1.27",
		Direct: []assets.Requirement{{Path: "example.test/web", Version: "v1.12.0"}},
		Indirect: []assets.Requirement{
			{Path: "example.test/cli", Version: "v1.10.2"},
			{Path: "example.test/mono/sub", Version: "v0.3.0"},
			{Path: "example.test/testkit", Version: "v1.11.1"},
			{Path: "example.test/yaml", Version: "v1.0.0"},
		},
	}
}

func TestPromoteImportedMovesTheIndirectRequiresTheCodeImports(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{"red imported by a command", map[string]string{"main.go": "package main\n\nimport _ \"example.test/cli/cobra\"\n"}, []string{"example.test/cli"}},
		{"red imported by a test", map[string]string{"x_test.go": "package main\n\nimport _ \"example.test/testkit/assert\"\n"}, []string{"example.test/testkit"}},
		{"red the longest module owns the import", map[string]string{"a.go": "package main\n\nimport _ \"example.test/mono/sub/pkg\"\n"}, []string{"example.test/mono/sub"}},
		{"near direct require imported", map[string]string{"a.go": "package main\n\nimport _ \"example.test/web\"\n"}, nil},
		{"near testdata, vendor, dot and nested module", map[string]string{
			"testdata/a.go": "package a\n\nimport _ \"example.test/yaml\"\n",
			"vendor/a.go":   "package a\n\nimport _ \"example.test/yaml\"\n",
			".hidden/a.go":  "package a\n\nimport _ \"example.test/yaml\"\n",
			"tools/go.mod":  "module example.test/birth/tools\n",
			"tools/a.go":    "package tools\n\nimport _ \"example.test/yaml\"\n",
		}, nil},
		{"near unparsable file", map[string]string{"broken.go": "package\n"}, nil},
		{"near own and standard packages", map[string]string{"a.go": "package main\n\nimport (\n\t_ \"fmt\"\n\t_ \"example.test/birth/internal/x\"\n)\n"}, nil},
		{"red negated platform tag", map[string]string{"gen.go": "//go:build !linux\n\npackage main\n\nimport _ \"example.test/cli\"\n"}, []string{"example.test/cli"}},
		{"red tools tag", map[string]string{"gen.go": "//go:build tools\n\npackage main\n\nimport _ \"example.test/cli\"\n"}, []string{"example.test/cli"}},
		{"near ignore tag", map[string]string{"gen.go": "//go:build ignore\n\npackage main\n\nimport _ \"example.test/cli\"\n"}, nil},
		{"near legacy ignore tag", map[string]string{"gen.go": "// +build ignore\n\npackage main\n\nimport _ \"example.test/cli\"\n"}, nil},
		{"near underscore file", map[string]string{"_gen.go": "package main\n\nimport _ \"example.test/cli\"\n"}, nil},
		{"near dot file", map[string]string{".gen.go": "package main\n\nimport _ \"example.test/cli\"\n"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			for path, body := range tc.files {
				writeBefore(t, filepath.Join(root, filepath.FromSlash(path)), body)
			}
			info, promoted, err := bootstrap.PromoteImported(root, importsModule())
			if err != nil {
				t.Fatal(err)
			}
			requireImported(t, info, promoted, tc.want)
		})
	}
}

func writeImportFile(t *testing.T, root, path, body string) {
	t.Helper()
	fullPath := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func requireImported(t *testing.T, info bootstrap.ModuleInfo, promoted []assets.Requirement, want []string) {
	t.Helper()
	got := make([]string, 0, len(promoted))
	for _, requirement := range promoted {
		got = append(got, requirement.Path)
		if !slices.Contains(info.Direct, requirement) || slices.Contains(info.Indirect, requirement) {
			t.Fatalf("%s stayed indirect: %+v", requirement.Path, info)
		}
	}
	if !slices.Equal(got, want) || !slices.IsSortedFunc(info.Direct, compareRequirement) {
		t.Fatalf("promoted = %v, want %v; direct %+v", got, want, info.Direct)
	}
}

func compareRequirement(a, b assets.Requirement) int {
	switch {
	case a.Path < b.Path:
		return -1
	case a.Path > b.Path:
		return 1
	}
	return 0
}

func TestPromoteImportedUsesTheLongestOwningModule(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, current, direct, indirect, imported string
		want                                      bool
	}{
		{"indirect child", "example.test/app", "example.test/mono", "example.test/mono/sub", "example.test/mono/sub/pkg", true},
		{"direct child", "example.test/app", "example.test/mono/sub", "example.test/mono", "example.test/mono/sub/pkg", false},
		{"own module beats ancestor", "example.test/mono/sub", "", "example.test/mono", "example.test/mono/sub/pkg", false},
		{"own module exact", "example.test/mono/sub", "", "example.test/mono", "example.test/mono/sub", false},
		{"required child beats own module", "example.test/mono", "", "example.test/mono/sub", "example.test/mono/sub/pkg", true},
		{"slash boundary", "example.test/app", "", "example.test/mono", "example.test/monolith/pkg", false},
		{"exact requirement", "example.test/app", "", "example.test/mono", "example.test/mono", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeBefore(t, filepath.Join(root, "a.go"), "package app\n\nimport _ \""+tc.imported+"\"\n")
			requirement := assets.Requirement{Path: tc.indirect, Version: "v1.0.0"}
			info := bootstrap.ModuleInfo{Path: tc.current, Indirect: []assets.Requirement{requirement}}
			if tc.direct != "" {
				info.Direct = []assets.Requirement{{Path: tc.direct, Version: "v1.0.0"}}
			}
			got, promoted, err := bootstrap.PromoteImported(root, info)
			if err != nil {
				t.Fatal(err)
			}
			if slices.Contains(promoted, requirement) != tc.want || len(promoted) > 1 ||
				slices.Contains(got.Direct, requirement) != tc.want || slices.Contains(got.Indirect, requirement) == tc.want {
				t.Fatalf("promoted = %+v, module = %+v, want promotion %v", promoted, got, tc.want)
			}
		})
	}
}

func TestPromoteImportedKeepsRootAndDirectoryBoundaries(t *testing.T) {
	t.Parallel()
	for _, name := range []string{".root", "_root"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := filepath.Join(t.TempDir(), name)
			for path, body := range map[string]string{
				"go.mod":       "module example.test/birth\n",
				"a.go":         "package app\n\nimport _ \"example.test/cli\"\n",
				"_tools/a.go":  "package tool\n\nimport _ \"example.test/yaml\"\n",
				"tools/go.mod": "invalid module document\n",
				"tools/a.go":   "package tool\n\nimport _ \"example.test/yaml\"\n",
				"empty/go.mod": "",
				"empty/a.go":   "package tool\n\nimport _ \"example.test/yaml\"\n",
				"README":       "package app\n\nimport _ \"example.test/yaml\"\n",
			} {
				if body == "" {
					writeImportFile(t, root, path, body)
				} else {
					writeBefore(t, filepath.Join(root, filepath.FromSlash(path)), body)
				}
			}
			_, promoted, err := bootstrap.PromoteImported(root, importsModule())
			want := []assets.Requirement{{Path: "example.test/cli", Version: "v1.10.2"}}
			if err != nil || !slices.Equal(promoted, want) {
				t.Fatalf("directory boundaries = %+v, %v; want %+v", promoted, err, want)
			}
		})
	}
}

func TestPromoteImportedBoundsSourceReads(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		extra int
		want  bool
	}{{"exact bound", 0, true}, {"over bound", 1, false}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			body := "package app\n\nimport _ \"example.test/cli\"\n//"
			body += strings.Repeat("x", model.MaxReadBytes+tc.extra-len(body))
			writeBefore(t, filepath.Join(root, "a.go"), body)
			_, promoted, err := bootstrap.PromoteImported(root, importsModule())
			if err != nil || (len(promoted) == 1) != tc.want || len(promoted) > 1 {
				t.Fatalf("bounded imports = %+v, %v; want promotion %v", promoted, err, tc.want)
			}
		})
	}
}

func TestPromoteImportedWrapsWalkFailures(t *testing.T) {
	t.Parallel()
	_, _, err := bootstrap.PromoteImported(filepath.Join(t.TempDir(), "missing"), importsModule())
	if !errors.Is(err, fs.ErrNotExist) || !strings.HasPrefix(err.Error(), "bootstrap: read the module imports: ") {
		t.Fatalf("walk error = %v", err)
	}
}

func TestPromoteImportedSkipsSymbolicLinks(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links require privileges on Windows")
	}
	root, outside := t.TempDir(), t.TempDir()
	writeBefore(t, filepath.Join(outside, "a.go"), "package app\n\nimport _ \"example.test/yaml\"\n")
	for name, target := range map[string]string{"linked.go": filepath.Join(outside, "a.go"), "linked": outside} {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	_, promoted, err := bootstrap.PromoteImported(root, importsModule())
	if err != nil || len(promoted) != 0 {
		t.Fatalf("symbolic links supplied imports: %+v, %v", promoted, err)
	}
}

func TestPromotedReportNamesTheDirectSetAndRemedy(t *testing.T) {
	t.Parallel()
	portuguese := "go.mod marca como indiretos módulos que o código importa diretamente: example.test/cli, example.test/yaml. " +
		"O registro herdado já os lista como dependências diretas; rode go mod tidy para que go.mod diga o mesmo.\n"
	english := "go.mod marks as indirect modules the code imports directly: example.test/cli, example.test/yaml. " +
		"The inherited baseline already lists them as direct dependencies; run go mod tidy so go.mod says the same.\n"
	for _, tc := range []struct{ language, want string }{{"", portuguese}, {"pt-BR", portuguese}, {"en", english}} {
		t.Run(tc.language, func(t *testing.T) {
			t.Parallel()
			plan := bootstrap.TemplatePlan{Language: tc.language, Promoted: []assets.Requirement{
				{Path: "example.test/cli", Version: "v1.0.0"}, {Path: "example.test/yaml", Version: "v2.0.0"},
			}}
			var out bytes.Buffer
			if err := bootstrap.WriteTemplateDiff(&out, plan); err != nil || out.String() != tc.want {
				t.Fatalf("report = %q, %v; want %q", out.String(), err, tc.want)
			}
			out.Reset()
			plan.Promoted = nil
			if err := bootstrap.WriteTemplateDiff(&out, plan); err != nil || out.Len() != 0 {
				t.Fatalf("empty report = %q, %v", out.String(), err)
			}
		})
	}
}

func TestPromotedReportFailureStopsBeforeTemplateWrites(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	plan, err := bootstrap.PlanTemplates(root, []assets.Rendered{templateFile("a.txt", "new\n", assets.OwnedByKit)})
	if err != nil {
		t.Fatal(err)
	}
	plan.Promoted = []assets.Requirement{{Path: "example.test/cli", Version: "v1.0.0"}}
	writer := &templateMutationWriter{fail: true, calls: 1}
	if err := bootstrap.ApplyTemplates(root, plan, writer); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("report error = %v", err)
	}
	requireTemplateAbsent(t, root, "a.txt")
}

func TestPromoteImportedSortsEveryResultWithoutChangingInput(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeBefore(t, filepath.Join(root, "a.go"), "package app\n\nimport (\n_ \"example.test/yaml\"\n_ \"example.test/cli\"\n)\n")
	info := importsModule()
	info.Direct = append(info.Direct, assets.Requirement{Path: "example.test/alpha", Version: "v1.0.0"})
	slices.Reverse(info.Indirect)
	beforeDirect, beforeIndirect := slices.Clone(info.Direct), slices.Clone(info.Indirect)
	got, promoted, err := bootstrap.PromoteImported(root, info)
	if err != nil {
		t.Fatal(err)
	}
	requireImported(t, got, promoted, []string{"example.test/cli", "example.test/yaml"})
	wantDirect := []assets.Requirement{
		{Path: "example.test/alpha", Version: "v1.0.0"},
		{Path: "example.test/cli", Version: "v1.10.2"},
		{Path: "example.test/web", Version: "v1.12.0"},
		{Path: "example.test/yaml", Version: "v1.0.0"},
	}
	wantIndirect := []assets.Requirement{
		{Path: "example.test/mono/sub", Version: "v0.3.0"}, {Path: "example.test/testkit", Version: "v1.11.1"},
	}
	if !slices.Equal(got.Direct, wantDirect) || !slices.Equal(got.Indirect, wantIndirect) {
		t.Fatalf("module = %+v; want direct %+v and indirect %+v", got, wantDirect, wantIndirect)
	}
	if !slices.Equal(info.Direct, beforeDirect) || !slices.Equal(info.Indirect, beforeIndirect) {
		t.Fatalf("input changed: %+v; before direct %+v and indirect %+v", info, beforeDirect, beforeIndirect)
	}
}
