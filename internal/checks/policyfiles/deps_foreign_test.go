package policyfiles_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

type protectedBaseCase struct {
	name, side string
	edit       func(*proof.Fixture)
	profile    string
	scan       bool
	want       []string
	status     model.StatusKind
	outside    bool
}

func TestCodeownersForeignUsesProtectedBase(t *testing.T) {
	t.Parallel()
	for _, tc := range []protectedBaseCase{
		{name: "candidate cannot authorize itself", side: "red", profile: "fixture", want: []string{"foreign-owned-change"}},
		{name: "catchall change near", side: "near", profile: "fixture"},
		{
			name: "delete foreign path", side: "red", profile: "fixture",
			edit: func(f *proof.Fixture) {
				f.Staged.Changes[1] = model.Change{
					Op: model.ChangeDelete, OldPath: ".github/check.yml",
					Before: "file", Mode: "100644",
				}
			},
			want: []string{"foreign-owned-change"},
		},
		{
			name: "rename out of foreign path", side: "red", profile: "fixture",
			edit: func(f *proof.Fixture) {
				f.Staged.Changes[1] = model.Change{
					Op: model.ChangeRename, OldPath: ".github/check.yml",
					NewPath: "src/check.yml", Before: "file", After: "file",
					Mode: "100644",
				}
			},
			want: []string{"foreign-owned-change"},
		},
		{
			name: "deleted ownership file still protects", side: "red", profile: "fixture",
			edit: func(f *proof.Fixture) {
				delete(f.Files, ".github/CODEOWNERS")
				f.Staged.Changes[0] = model.Change{
					Op: model.ChangeDelete, OldPath: ".github/CODEOWNERS",
					Before: "base", Mode: "100644",
				}
			},
			want: []string{"foreign-owned-change"},
		},
		{
			name: "missing base is not a finding", side: "red", profile: "fixture",
			edit:   func(f *proof.Fixture) { delete(f.Staged.Blobs, "base") },
			status: model.StatusUnmeasured,
		},
		{
			name: "unsupported base pattern", side: "red", profile: "fixture",
			edit: func(f *proof.Fixture) {
				f.Staged.Blobs["base"] = "* @app\n/.github/[ab]* @platform\n"
			},
			status: model.StatusUnmeasured, outside: true,
		},
		{name: "profile off", side: "red", status: model.StatusNotApplicable},
		{name: "scan has no staged judgment", side: "red", profile: "fixture", scan: true, status: model.StatusNotApplicable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.verify(t)
		})
	}
}

func (tc protectedBaseCase) verify(t *testing.T) {
	t.Helper()
	f, err := proof.LoadFixture("../../proof/testdata/policy.codeowners-foreign/" + tc.side)
	if err != nil {
		t.Fatal(err)
	}
	if tc.edit != nil {
		tc.edit(&f)
	}
	in, err := f.Input(model.ScopeTree, proof.PolicyFor(proof.Entry{Profile: tc.profile}))
	if err != nil {
		t.Fatal(err)
	}
	if tc.scan {
		in.Mode = model.ModeScan
	}
	builder := engine.New()
	if registerErr := builder.Register(policyfiles.CodeownersForeign()); registerErr != nil {
		t.Fatal(registerErr)
	}
	registry, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	run, err := engine.Dispatch(t.Context(), registry, in)
	if err != nil {
		t.Fatal(err)
	}
	got := run.Outcomes[0].Result
	wantStatus := tc.status
	if wantStatus == 0 {
		wantStatus = model.StatusMeasured
	}
	if got.Status.Kind != wantStatus || !slices.Equal(codes(got), tc.want) ||
		model.IsOutsideSubset(got.Status) != tc.outside {
		t.Fatalf("result = %+v", got)
	}
	if len(tc.want) != 0 &&
		!strings.Contains(got.Findings[0].Message, ".github/check.yml") {
		t.Fatalf("finding does not name the changed path: %+v", got)
	}
}
