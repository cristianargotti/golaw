package policyfiles_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

// nightly is the scheduled lane of the near fixture, kept by name so a row
// can remove it.
const nightly = ".github/workflows/nightly.yml"

// TestProfileTargets pins policy.profile-targets (contract D15): every
// target the profile adds is reached from ci, directly or through a
// target of the file, or run by a scheduled or manually dispatched lane;
// an absent Makefile and an absent ci target are the core checks' to report.
func TestProfileTargets(t *testing.T) {
	t.Parallel()
	const id = "policy.profile-targets"
	data := profiletest.Data(t)
	red := depsProof(t, id, "red", "go")
	red.data = data
	near := depsProof(t, id, "near", "go")
	near.data = data
	staged := red
	staged.mode = model.ModeStaged
	makefile := func(f fixture, content string) fixture {
		return depsEdit(depsEdit(f, nightly, ""), "Makefile", ".PHONY: ci\nhooks-test image-lint:\n"+content)
	}
	assertDeps(t, policyfiles.ProfileTargets(), []depsCase{
		{name: "target outside ci red", input: red, want: []string{"profile-target-missing"}},
		{name: "through a prerequisite and a scheduled lane near", input: near},
		{name: "index red", input: staged, want: []string{"profile-target-missing"}},
		{name: "direct prerequisites near", input: makefile(near, "ci: hooks-test image-lint\n")},
		{name: "both outside ci red", input: makefile(near, "ci:\n"), want: []string{"profile-target-missing", "profile-target-missing"}},
		{
			name:  "push lane red",
			input: depsEdit(makefile(near, "ci: hooks-test\n"), nightly, "on: push\njobs:\n  image:\n    steps:\n      - run: make image-lint\n"),
			want:  []string{"profile-target-missing"},
		},
		{
			name:   "dynamic lane",
			input:  depsEdit(makefile(near, "ci: hooks-test\n"), nightly, "on: workflow_dispatch\njobs:\n  image:\n    steps:\n      - run: make $TARGET\n"),
			status: model.StatusUnmeasured, reason: nightly, outside: true,
		},
		{
			name:   "cycle before the target",
			input:  makefile(near, "ci: checks\nchecks: ci\n"),
			status: model.StatusUnmeasured, reason: "cyclic prerequisite ci", outside: true,
		},
		{
			name:   "over the read bound",
			input:  depsEdit(near, "Makefile", strings.Repeat("#", model.MaxReadBytes+1)),
			status: model.StatusUnmeasured, reason: model.ErrTooLarge.Error(),
		},
		{name: "no Makefile near", input: depsEdit(depsEdit(near, "Makefile", ""), nightly, "")},
		{name: "no ci target near", input: makefile(near, "lint:\n")},
	})
}

// TestProfileTargetsAppliesOnlyWithItsSection pins the gates of D5.
func TestProfileTargetsAppliesOnlyWithItsSection(t *testing.T) {
	t.Parallel()
	red := depsProof(t, "policy.profile-targets", "red", "go")
	withData := func(f fixture, d *model.ProfileData) fixture { f.data = d; return f }
	noSection := *profiletest.Data(t)
	noSection.Targets = nil
	noGo := withData(red, profiletest.Data(t))
	noGo.capabilities = nil
	assertDeps(t, policyfiles.ProfileTargets(), []depsCase{
		{name: "core only", input: red, status: model.StatusNotApplicable, reason: "no profile is enrolled"},
		{name: "no targets section", input: withData(red, &noSection), status: model.StatusNotApplicable, reason: "the profile carries no targets"},
		{name: "go capability off", input: noGo, status: model.StatusNotApplicable, reason: "capability go is not enabled"},
	})
}
