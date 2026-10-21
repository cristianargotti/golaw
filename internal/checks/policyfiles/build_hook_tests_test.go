package policyfiles_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestHookTestsAreProfileOnlyAndPairExactScripts(t *testing.T) {
	t.Parallel()
	const script = "scripts/hooks/check.sh"
	const testFile = "scripts/hooks/check.test.sh"
	const shell = "#!/bin/sh\nprintf 'ok\\n'\n"
	const testBody = "#!/bin/sh\ntest \"$(sh scripts/hooks/check.sh)\" = ok\n"
	for _, tc := range []struct {
		name, profile string
		files         map[string]string
		want          bool
		kind          model.StatusKind
	}{
		{"red missing test", "fixture", map[string]string{script: shell}, true, model.StatusMeasured},
		{"near paired", "fixture", map[string]string{script: shell, testFile: testBody}, false, model.StatusMeasured},
		{"near core", "", map[string]string{script: shell}, false, model.StatusNotApplicable},
		{"near no scripts", "fixture", nil, false, model.StatusMeasured},
		{"near test is not script", "fixture", map[string]string{testFile: testBody}, false, model.StatusMeasured},
		{"near unrelated shell", "fixture", map[string]string{"scripts/release.sh": shell}, false, model.StatusMeasured},
		{"near another extension", "fixture", map[string]string{"scripts/hooks/check.py": "print('ok')\n"}, false, model.StatusMeasured},
		{"near nested script not judged", "fixture", map[string]string{"scripts/hooks/checks/check.sh": shell}, false, model.StatusMeasured},
		{"red wrong sibling", "fixture", map[string]string{script: shell, "scripts/hooks/other.test.sh": testBody}, true, model.StatusMeasured},
		{"near nested pair", "fixture", map[string]string{
			"scripts/hooks/checks/check.sh":      shell,
			"scripts/hooks/checks/check.test.sh": testBody,
		}, false, model.StatusMeasured},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := measure(t, policyfiles.HookTests(), fixture{
				files: tc.files, profile: tc.profile,
			})
			if got.Status.Kind != tc.kind {
				t.Fatalf("status = %+v", got.Status)
			}
			if slices.Contains(codes(got), "hook-test-missing") != tc.want {
				t.Fatalf("findings = %+v", got.Findings)
			}
			if tc.want && (got.Findings[0].Path == "" || got.Findings[0].Message == "") {
				t.Fatalf("missing repair: %+v", got.Findings[0])
			}
		})
	}
}

func TestHookTestsCorePolicyDoesNotAccessSnapshot(t *testing.T) {
	t.Parallel()
	policy := model.CoreDefaults()
	got, err := policyfiles.HookTests().Run(t.Context(), model.Input{Policy: &policy})
	want := model.NotApplicable("no profile is enrolled; the profile turns this rule on")
	if err != nil || got.Status != want || len(got.Findings) != 0 || len(got.Unread) != 0 {
		t.Fatalf("core result = %+v, %v", got, err)
	}
}

func TestHookTestsReadFailuresKeepTheActualPath(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"scripts/hooks/check.sh", "scripts/hooks/check.test.sh"} {
		for _, failure := range []error{model.ErrTooLarge, model.ErrUnreadable} {
			t.Run(path+"/"+failure.Error(), func(t *testing.T) {
				t.Parallel()
				policy := model.CoreDefaults()
				policy.Profile = "fixture"
				tree := proof.NewTree(map[string][]byte{
					"scripts/hooks/check.sh":      nil,
					"scripts/hooks/check.test.sh": nil,
				})
				got, err := policyfiles.HookTests().Run(t.Context(), model.Input{
					Policy: &policy, Snapshot: buildReadFailure{
						Snapshot: tree, path: path, err: fmt.Errorf("read %s: %w", path, failure),
					},
				})
				if err != nil || len(got.Findings) != 0 {
					t.Fatalf("failed read = %+v, %v", got, err)
				}
				requireMakefileReadFailure(t, got, path, failure)
			})
		}
	}
}

func TestHookTestsMissingPairsHaveExactRepairs(t *testing.T) {
	t.Parallel()
	policy := model.CoreDefaults()
	policy.Profile = "fixture"
	tree := proof.NewTree(map[string][]byte{
		"scripts/hooks/alpha.sh":        nil,
		"scripts/hooks/check.sh.sh":     nil,
		"scripts/hooks/paired.sh":       nil,
		"scripts/hooks/paired.test.sh":  nil,
		"scripts/hooks/another.test.sh": nil,
	})
	got, err := policyfiles.HookTests().Run(t.Context(), model.Input{Policy: &policy, Snapshot: tree})
	want := []model.Finding{
		{
			Path: "scripts/hooks/alpha.sh", Code: "hook-test-missing", Severity: model.SeverityDeny,
			Message: "add scripts/hooks/alpha.test.sh beside scripts/hooks/alpha.sh",
		},
		{
			Path: "scripts/hooks/check.sh.sh", Code: "hook-test-missing", Severity: model.SeverityDeny,
			Message: "add scripts/hooks/check.sh.test.sh beside scripts/hooks/check.sh.sh",
		},
	}
	if err != nil || got.Status != model.Measured() || !slices.Equal(got.Findings, want) {
		t.Fatalf("missing pair repairs = %+v, %v", got, err)
	}
}

func TestHookTestsAcceptExistingTestsWithoutGradingTheirContents(t *testing.T) {
	t.Parallel()
	for _, body := range []string{"", "not executable shell syntax: ((("} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()
			got := measure(t, policyfiles.HookTests(), fixture{
				profile: "fixture",
				files: map[string]string{
					"scripts/hooks/check.sh":      "not shell syntax either",
					"scripts/hooks/check.test.sh": body,
				},
			})
			if got.Status != model.Measured() || len(got.Findings) != 0 || len(got.Unread) != 0 {
				t.Fatalf("existing test result = %+v", got)
			}
		})
	}
}

func TestHookTestsSelectOnlyDirectScriptsAndReadTheirPairs(t *testing.T) {
	t.Parallel()
	policy := model.CoreDefaults()
	policy.Profile = "fixture"
	snapshot := &squawkSnapshot{Tree: proof.NewTree(map[string][]byte{
		"scripts/hooks/check.sh":             nil,
		"scripts/hooks/check.test.sh":        nil,
		"scripts/hooks/other.test.sh":        nil,
		"scripts/hooks/nested/check.sh":      nil,
		"scripts/hooks/nested/check.test.sh": nil,
		"scripts/hooks/check.py":             nil,
		"scripts/hookscript/check.sh":        nil,
		"scripts/release.sh":                 nil,
	})}
	got, err := policyfiles.HookTests().Run(t.Context(), model.Input{Policy: &policy, Snapshot: snapshot})
	if err != nil || got.Status != model.Measured() || len(got.Findings) != 0 || len(got.Unread) != 0 {
		t.Fatalf("selected pairs = %+v, %v", got, err)
	}
	wantReads := []string{"scripts/hooks/check.sh", "scripts/hooks/check.test.sh"}
	if snapshot.listed || !snapshot.selected || !slices.Equal(snapshot.reads, wantReads) {
		t.Fatalf("snapshot access = %+v", snapshot)
	}
}
