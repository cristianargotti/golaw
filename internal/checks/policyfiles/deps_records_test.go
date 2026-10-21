package policyfiles_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestDependencyRecords(t *testing.T) {
	t.Parallel()
	const id = "policy.dependency-records"
	const record = "docs/adr/0001-library.md"
	const missing = "dependency-record-missing"
	const localModule = "module example.test/app\n" +
		"require example.test/a v1.0.0\nreplace example.test/a => ./local\n"
	const localChild = "module example.test/a\nrequire example.test/child v1.0.0\n"
	const both = "---\nrecord: 1\nstatus: accepted\n" +
		"modules: [example.test/a, example.test/child]\n---\n"
	red := depsProof(t, id, "red", "go")
	near := depsProof(t, id, "near", "go")
	local := depsEdit(depsEdit(near, "go.mod", localModule), "local/go.mod", localChild)
	local = depsEdit(local, record, "---\nmodules: [example.test/a]\n---\n")
	custom := depsEdit(near, record, "")
	custom.layout = &model.Layout{Main: model.DefaultMain, Decisions: "docs/decisions"}
	custom = depsEdit(custom, "docs/decisions/0001-library.md", near.files[record])
	tools := depsEdit(near, "tools/go.mod",
		"module example.test/app/tools\nrequire example.test/tool v1.0.0\n")
	assertDeps(t, policyfiles.DependencyRecords(), []depsCase{
		{name: "second block red", input: red, want: []string{missing}},
		{name: "exact front matter near", input: near},
		{name: "configured directory", input: custom},
		{
			name:  "converted line endings near",
			input: depsEdit(near, record, strings.ReplaceAll(near.files[record], "\n", "\r\n")),
		},
		{
			name: "fixture module is not an application module",
			input: depsEdit(near, "internal/x/testdata/m/go.mod",
				"module example.test/fixture\nrequire example.test/unrecorded v1.0.0\n"),
		},
		{
			name: "prefix is not exact",
			input: depsEdit(near, record,
				"---\nmodules: [example.test/a, example.test/b/extra]\n---\n"),
			want: []string{missing},
		},
		{
			name: "body is not front matter",
			input: depsEdit(near, record,
				"---\nmodules: [example.test/a]\n---\n\nexample.test/b\n"),
			want: []string{missing},
		},
		{
			name: "indirect requires no record",
			input: fixture{capabilities: []string{"go"}, files: map[string]string{
				"go.mod": "module example.test/app\nrequire example.test/a v1.0.0 // indirect\n",
			}},
		},
		{name: "follow internal replacement", input: local, want: []string{missing}},
		{name: "internal replacement recorded", input: depsEdit(local, record, both)},
		{
			name: "external replacement",
			input: depsEdit(local, "go.mod",
				strings.ReplaceAll(localModule, "./local", "../outside")),
			status: model.StatusUnmeasured, reason: "go.mod:3:", outside: true,
		},
		{name: "tool dependency red", input: tools, want: []string{missing}},
		{
			name: "tool dependency near",
			input: depsEdit(tools, record,
				"---\nmodules: [example.test/a, example.test/b, example.test/tool]\n---\n"),
		},
		{
			name: "workspace limit", input: depsEdit(near, "go.work", "go 1.27\nuse .\n"),
			status: model.StatusUnmeasured, reason: "go.work", outside: true,
		},
		{
			name: "vendor limit", input: depsEdit(near, "vendor/modules.txt", "# example.test/a v1.0.0\n"),
			status: model.StatusUnmeasured, reason: "vendor/", outside: true,
		},
		{
			name:   "unrelated application module",
			input:  depsEdit(near, "other/go.mod", "module example.test/other\n"),
			status: model.StatusUnmeasured, reason: "other/go.mod", outside: true,
		},
		{
			name:   "broken front matter",
			input:  depsEdit(near, record, "---\nmodules: [\n---\n"),
			status: model.StatusUnmeasured, reason: record,
		},
		{
			name:   "read bound",
			input:  depsEdit(near, "go.mod", strings.Repeat("x", model.MaxReadBytes+1)),
			status: model.StatusUnmeasured, reason: model.ErrTooLarge.Error(),
		},
		{
			name: "capability off", input: depsProof(t, id, "red"),
			status: model.StatusNotApplicable, reason: "capability go",
		},
	})
}

func TestDependencyRecordsNamesEachRequirementAndMeasuredModule(t *testing.T) {
	t.Parallel()
	input := fixture{capabilities: []string{"go"}, files: map[string]string{
		"go.mod": "module example.test/app\nrequire example.test/a v1.0.0\n" +
			"replace example.test/a => ./local\n",
		"local/go.mod": "module example.test/a\nrequire example.test/child v1.0.0\n",
		"tools/go.mod": "module example.test/tools\nrequire example.test/tool v1.0.0\n",
	}}
	got := measure(t, policyfiles.DependencyRecords(), input)
	locations := make([]string, 0, len(got.Findings))
	for _, found := range got.Findings {
		locations = append(locations, fmt.Sprintf("%s:%d:%s", found.Path, found.Line, found.Message))
	}
	slices.Sort(locations)
	want := []string{
		"go.mod:2:record example.test/a in the modules front matter of a decision under docs/adr",
		"local/go.mod:2:record example.test/child in the modules front matter of a decision under docs/adr",
		"tools/go.mod:2:record example.test/tool in the modules front matter of a decision under docs/adr",
	}
	if got.Status.Kind != model.StatusMeasured || !slices.Equal(locations, want) {
		t.Fatalf("result = %+v, want requirements %v", got, want)
	}
	for _, text := range []string{"go.mod", "local/go.mod", "tools/go.mod", "dependency mode: modules"} {
		if !strings.Contains(got.Scope, text) {
			t.Errorf("scope = %q, want %q", got.Scope, text)
		}
	}
}

func TestDependencyRecordsWaitsForCompleteEvidence(t *testing.T) {
	t.Parallel()
	input := fixture{capabilities: []string{"go"}, files: map[string]string{
		"go.mod": "module example.test/app\nrequire example.test/unrecorded v1.0.0\n",
	}}
	const record = "docs/adr/0001-library.md"
	assertDeps(t, policyfiles.DependencyRecords(), []depsCase{
		{
			name: "missing root module is a stated limit", input: depsEdit(input, "go.mod", ""),
			status: model.StatusUnmeasured, reason: "go.mod:1:", outside: true,
		},
		{
			name: "record over read bound", input: depsEdit(input, record, strings.Repeat("x", model.MaxReadBytes+1)),
			status: model.StatusUnmeasured, reason: record,
		},
		{
			name: "invalid record shape", input: depsEdit(input, record, "---\nmodules: example.test/unrecorded\n---\n"),
			status: model.StatusUnmeasured, reason: record + ":2:", outside: true,
		},
		{
			name: "incomplete record", input: depsEdit(input, record, "---\nmodules: [example.test/unrecorded]\n"),
			status: model.StatusUnmeasured, reason: record + ":1:", outside: true,
		},
		{
			name:  "records outside decisions directory",
			input: depsEdit(input, "docs/adr-old/0001-library.md", "---\nmodules: [example.test/unrecorded]\n---\n"),
			want:  []string{"dependency-record-missing"},
		},
		{
			name:  "extra records allowed",
			input: depsEdit(input, record, "---\nmodules: [example.test/unrecorded, example.test/extra]\n---\n"),
		},
	})
}

func TestDependencyRecordsNamesUnsupportedGraphEvidence(t *testing.T) {
	t.Parallel()
	const app = "module example.test/app\nrequire example.test/a v1.0.0\n"
	input := fixture{capabilities: []string{"go"}, files: map[string]string{"go.mod": app}}
	rows := []struct {
		name   string
		input  fixture
		reason string
	}{
		{
			name: "missing root", input: depsEdit(input, "go.mod", ""),
			reason: "go.mod:1: no go.mod at the repository root, which the go capability needs; " +
				"restore it or remove go from the capabilities",
		},
		{
			name:  "second application module",
			input: depsEdit(input, "other/go.mod", "module example.test/other\n"),
			reason: "other/go.mod:1: a second application module; " +
				"golaw measures one module at the root, tools/go.mod and the local replacements they name",
		},
		{
			name:  "missing local replacement",
			input: depsEdit(input, "go.mod", app+"replace example.test/a => ./local\n"),
			reason: "go.mod:3: replace example.test/a => ./local " +
				"names a directory without a go.mod in the snapshot",
		},
		{
			name: "missing tools replacement",
			input: depsEdit(input, "tools/go.mod", "module example.test/tools\n"+
				"require example.test/a v1.0.0\nreplace example.test/a => ../local\n"),
			reason: "tools/go.mod:3: replace example.test/a => ../local " +
				"names a directory without a go.mod in the snapshot",
		},
	}
	for index := range rows {
		row := &rows[index]
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got := measure(t, policyfiles.DependencyRecords(), row.input)
			want := row.reason + ": " + model.ErrOutsideSubset.Error()
			if got.Status.Kind != model.StatusUnmeasured || !model.IsOutsideSubset(got.Status) ||
				got.Status.Reason != want || len(got.Findings) != 0 {
				t.Fatalf("result = %+v, want no findings and exact outside reason %q", got, want)
			}
		})
	}
}

func TestDependencyRecordsNamesRecordFailuresOnce(t *testing.T) {
	t.Parallel()
	const record = "docs/adr/x.md"
	input := fixture{capabilities: []string{"go"}, files: map[string]string{
		"go.mod": "module example.test/app\nrequire example.test/unrecorded v1.0.0\n",
	}}
	for _, row := range []struct {
		name    string
		data    string
		prefix  string
		outside bool
	}{
		{
			name: "malformed YAML", data: "---\nmodules: [\n---\n",
			prefix: "parse " + record + ":",
		},
		{
			name: "unsupported shape", data: "---\nmodules: example.test/unrecorded\n---\n",
			prefix: record + ":2:", outside: true,
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got := measure(t, policyfiles.DependencyRecords(), depsEdit(input, record, row.data))
			if got.Status.Kind != model.StatusUnmeasured || model.IsOutsideSubset(got.Status) != row.outside ||
				len(got.Findings) != 0 {
				t.Fatalf("result = %+v, want no findings and outside %v", got, row.outside)
			}
			if !strings.HasPrefix(got.Status.Reason, row.prefix) || strings.Count(got.Status.Reason, record) != 1 {
				t.Fatalf("reason = %q, want prefix %q and the record path once", got.Status.Reason, row.prefix)
			}
		})
	}
}
