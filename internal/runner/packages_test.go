package runner_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

func packageJSON(t *testing.T, root string, diagnostic bool) string {
	t.Helper()
	pkg := map[string]any{
		"Dir":        filepath.Join(root, "leaf"),
		"ImportPath": "example.test/graph/leaf",
		"Name":       "leaf",
		"Module": map[string]any{
			"Path": "example.test/graph", "Dir": root, "Main": true,
		},
	}
	if diagnostic {
		pkg["Error"] = map[string]string{"Err": "package diagnostic"}
	}
	data, err := json.Marshal(pkg)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestAffectedReadsOneCompletePackageStream(t *testing.T) {
	root := t.TempDir()
	valid := packageJSON(t, root, false)
	for _, tc := range []struct {
		name   string
		output string
		code   int
		bad    bool
	}{
		{"near-complete", valid, 0, false},
		{"near-usable-diagnostic", packageJSON(t, root, true), 0, false},
		{"near-exact-bound", strings.Repeat(" ", runner.MaxOutputBytes-len(valid)) + valid, 0, false},
		{"red-large-stream", strings.Repeat(" ", runner.MaxOutputBytes) + valid, 0, false},
		{"red-truncated", valid[:len(valid)-1], 0, true},
		{"red-trailing-json", valid + "\n{", 0, true},
		{"red-exit", valid, 2, true},
		{"red-no-identity", `{"Name":"leaf"}`, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			runnertest.Install(t, dir, "go", runnertest.Behavior{
				Stdout: tc.output, Stderr: "package listing failed\n",
				Code: tc.code,
			})
			t.Setenv("PATH", dir)
			got, err := runner.Affected(
				t.Context(), root, []string{"leaf/value.go"}, nil,
			)
			if tc.bad {
				if !errors.Is(err, runner.ErrPackages) {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if err != nil || !slices.Equal(got.Patterns, []string{"./leaf"}) {
				t.Fatalf("affected = %+v, %v", got, err)
			}
			want := [][]string{{"list", "-e", "-test", "-json", "./..."}}
			calls := runnertest.Calls(t, dir, "go")
			if !slices.EqualFunc(calls, want, slices.Equal) {
				t.Fatalf("calls = %v", calls)
			}
		})
	}
}

func TestAffectedPreservesStartFailure(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := runner.Affected(t.Context(), t.TempDir(), []string{"a.go"}, nil)
	if !errors.Is(err, runner.ErrPackages) || !errors.Is(err, runner.ErrStart) {
		t.Fatalf("error = %v", err)
	}
}

func TestAffectedPreservesCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := runner.Affected(ctx, t.TempDir(), []string{"a.go"}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}

func graphPackage(t *testing.T, identity, owner string, imports []string) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"ImportPath": identity, "ForTest": owner, "Imports": imports,
		"Name": "leaf", "Dir": "unrelated spelling",
		"Module": map[string]any{"Path": "example.test/graph", "Main": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestAffectedCanonicalizesTestGraphs(t *testing.T) {
	const module = "example.test/graph"
	leaf, middle, owner := module+"/leaf", module+"/middle", module+"/owner"
	base := graphPackage(t, leaf, "", nil)
	parent := graphPackage(t, middle, "", nil)
	tested := graphPackage(t, owner, "", nil)
	variant := graphPackage(t, middle+" ["+owner+".test]", owner, []string{leaf})
	external := graphPackage(t, owner+"_test ["+owner+".test]", owner, []string{middle + " [" + owner + ".test]"})
	binary := `{"ImportPath":"example.test/graph/owner.test","Name":"main","Imports":["example.test/graph/owner"],"Module":{"Path":"example.test/graph","Main":true}}`
	realMain := `{"ImportPath":"example.test/graph/real.test","Name":"main","Match":["./..."],"Module":{"Path":"example.test/graph","Main":true}}`
	collision := strings.ReplaceAll(realMain, "real.test", "owner.test")
	collision = strings.Replace(collision, `"Match"`, `"Imports":["example.test/graph/leaf"],"Match"`, 1)
	foreign := `{"ImportPath":"example.test/graph/tools","Name":"tools","Module":{"Path":"example.test/graph/tools","Main":false}}`
	for _, tc := range []struct {
		name     string
		output   string
		paths    []string
		patterns []string
		all      bool
	}{
		{
			"red-decorated-test-closure", base + parent + tested + variant + external + binary,
			[]string{"leaf/value.go"},
			[]string{"./leaf", "./middle", "./owner"},
			false,
		},
		{
			"near-recompiled-owner", base + parent + tested + variant + external,
			[]string{"owner/value.go"},
			[]string{"./owner"},
			false,
		},
		{
			"red-unlisted-decorated-copy", base + parent + graphPackage(t, owner, "", []string{middle + " [" + owner + ".test]"}),
			[]string{"middle/value.go"},
			[]string{"./middle", "./owner"},
			false,
		},
		{
			"near-generated-binary", base + tested + binary,
			[]string{"owner.test/main.go"},
			nil, false,
		},
		{
			"near-real-main-suffix", base + realMain,
			[]string{"real.test/main.go"},
			[]string{"./real.test"},
			false,
		},
		{
			"red-root-package", graphPackage(t, module, "", nil) + base,
			[]string{"value.go"},
			[]string{"."},
			false,
		},
		{
			"red-root-testdata", graphPackage(t, module, "", nil) + base,
			[]string{"testdata/case/input"},
			[]string{"."},
			false,
		},
		{
			"near-module-boundary", base + foreign + graphPackage(t, owner, "", []string{module + "/tools"}),
			[]string{"tools/value.go"},
			nil, false,
		},
		{
			"red-all-with-foreign", base + foreign + tested + binary,
			[]string{"go.mod"},
			[]string{"./..."},
			true,
		},
		{"red-binary-collision-after", base + tested + collision + binary, []string{"leaf/value.go"}, []string{"./leaf", "./owner.test"}, false},
		{"red-binary-collision-before", base + tested + binary + collision, []string{"leaf/value.go"}, []string{"./leaf", "./owner.test"}, false},
		{"near-binary-imports", base + tested + binary + collision, []string{"owner/value.go"}, []string{"./owner"}, false},
		{"near-empty-paths", "malformed", nil, nil, false},
		{"near-empty-module", "", []string{"go.mod"}, []string{"./..."}, true},
		{"near-empty-sums", "", []string{"go.sum"}, []string{"./..."}, true},
		{"near-empty-document", "", []string{"docs/readme.md"}, nil, false},
		{"near-import-without-identity", graphPackage(t, leaf, "", []string{""}), []string{"leaf/value.go"}, []string{"./leaf"}, false},
		{"near-unresolved-source-imports", base + graphPackage(t, owner, "", []string{"./leaf", module + "/leaf/../leaf"}), []string{"leaf/value.go"}, []string{"./leaf"}, false},
		{"near-outside-root", base, []string{"../leaf/value.go", "/leaf/value.go"}, nil, false},
		{"near-empty-name", graphPackage(t, module, "", nil), []string{""}, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			runnertest.Install(t, dir, "go", runnertest.Behavior{Stdout: tc.output})
			t.Setenv("PATH", dir)
			got, err := runner.Affected(t.Context(), t.TempDir(), tc.paths, nil)
			if err != nil || got.All != tc.all || !slices.Equal(got.Patterns, tc.patterns) {
				t.Fatalf("affected = %+v, %v", got, err)
			}
			if tc.output == "" && len(got.Packages) != 0 {
				t.Fatalf("empty stream packages = %v", got.Packages)
			}
			want := 1
			if len(tc.paths) == 0 {
				want = 0
			}
			if calls := runnertest.Calls(t, dir, "go"); len(calls) != want {
				t.Fatalf("go calls = %v, want %d", calls, want)
			}
		})
	}
}

func TestAffectedRejectsIncompleteGraphMetadata(t *testing.T) {
	const module = "example.test/graph"
	valid := graphPackage(t, module+"/leaf", "", nil)
	for _, tc := range []struct{ name, output string }{
		{"red-null", valid + "null"},
		{"red-missing-module", valid + `{"ImportPath":"example.test/graph/other"}`},
		{"red-multiple-main-modules", valid + `{"ImportPath":"other.test/p","Module":{"Path":"other.test","Main":true}}`},
		{"red-invalid-module", `{"ImportPath":"p","Module":{"Path":"../invalid","Main":true}}`},
		{"red-outside-module", valid + graphPackage(t, "other.test/p", "", nil)},
		{"red-variant-without-base", graphPackage(t, module+"/leaf ["+module+"/leaf.test]", module+"/leaf", nil)},
		{"red-owner-mismatch", valid + graphPackage(t, module+"/leaf [wrong.test]", module+"/leaf", nil)},
		{"red-invalid-test-owner", valid + graphPackage(t, module+"/leaf", "../owner", nil)},
		{"red-empty-test-owner", graphPackage(t, module+"/leaf [.test]", "", nil)},
		{"red-missing-test-owner", valid + graphPackage(t, module+"/leaf ["+module+"/missing.test]", module+"/missing", nil)},
		{"red-missing-import-owner", valid + graphPackage(t, module+"/owner", "", []string{module + "/leaf [" + module + "/missing.test]"})},
		{"red-relative-test-import", graphPackage(t, module+"/leaf", "", []string{"./leaf [" + module + "/leaf.test]"})},
		{"red-unresolved-import-variant", graphPackage(t, module+"/leaf", "", []string{module + "/gone [" + module + "/leaf.test]"})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			runnertest.Install(t, dir, "go", runnertest.Behavior{Stdout: tc.output})
			t.Setenv("PATH", dir)
			_, err := runner.Affected(t.Context(), t.TempDir(), []string{"leaf/value.go"}, nil)
			if !errors.Is(err, runner.ErrPackages) {
				t.Fatalf("incomplete graph = %v", err)
			}
		})
	}
}
