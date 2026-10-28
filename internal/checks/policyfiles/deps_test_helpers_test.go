package policyfiles_test

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

type depsCase struct {
	name    string
	input   fixture
	want    []string
	status  model.StatusKind
	reason  string
	outside bool
}

func depsProof(t *testing.T, id, side string, capabilities ...string) fixture {
	t.Helper()
	loaded, err := proof.LoadFixture("../../proof/testdata/" + id + "/" + side)
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string]string, len(loaded.Files))
	for path, data := range loaded.Files {
		files[path] = string(data)
	}
	return fixture{files: files, capabilities: capabilities}
}

func depsEdit(f fixture, path, content string) fixture {
	f.files = maps.Clone(f.files)
	if content == "" {
		delete(f.files, path)
	} else {
		f.files[path] = content
	}
	return f
}

func assertDeps(t *testing.T, check model.Check, rows []depsCase) {
	t.Helper()
	for i := range rows {
		row := &rows[i]
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got := measure(t, check, row.input)
			assertDepsResult(t, check, row, got)
		})
	}
}

func assertDepsResult(t *testing.T, check model.Check, row *depsCase, got model.Result) {
	t.Helper()
	wantStatus := row.status
	if wantStatus == 0 {
		wantStatus = model.StatusMeasured
	}
	if got.Status.Kind != wantStatus || !slices.Equal(codes(got), row.want) {
		t.Fatalf("result = %+v, want status %s and codes %v",
			got, wantStatus, row.want)
	}
	if !strings.Contains(got.Status.Reason, row.reason) ||
		model.IsOutsideSubset(got.Status) != row.outside {
		t.Fatalf("status = %+v, want reason %q, outside %v",
			got.Status, row.reason, row.outside)
	}
	if wantStatus != model.StatusMeasured && len(got.Findings) != 0 {
		t.Fatalf("unsettled result carries findings: %+v", got)
	}
	for k := range got.Findings {
		finding := &got.Findings[k]
		if finding.Check != check.Spec().ID || finding.Rule != check.Spec().Rule ||
			finding.Message == "" || finding.Path == "" {
			t.Fatalf("incomplete finding: %+v", *finding)
		}
	}
}
