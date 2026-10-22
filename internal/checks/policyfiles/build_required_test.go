package policyfiles_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

func buildTree(capabilities []string, omit string) map[string]string {
	targets := policyfiles.RequiredTargets(capabilities)
	targets = slices.DeleteFunc(targets, func(value string) bool {
		return value == omit
	})
	return map[string]string{"Makefile": strings.Join(targets, " ") + ":\n"}
}

func TestMakefileChecksOnlyApplicableRequirements(t *testing.T) {
	t.Parallel()
	const root = "Makefile"
	goCaps := []string{"go"}
	for _, tc := range []struct {
		name  string
		files map[string]string
		caps  []string
		code  string
		kind  model.StatusKind
	}{
		{"red absent", nil, goCaps, "make-file-missing", model.StatusMeasured},
		{"red missing lint", buildTree(goCaps, "lint"), goCaps, "make-target-missing", model.StatusMeasured},
		{"near complete", buildTree(goCaps, ""), goCaps, "", model.StatusMeasured},
		{"near no go", nil, nil, "", model.StatusNotApplicable},
		{"red db enabled", buildTree(goCaps, ""), []string{"db", "go"}, "make-target-missing", model.StatusMeasured},
		{"near db disabled", buildTree(goCaps, ""), goCaps, "", model.StatusMeasured},
		{"red phony alone", map[string]string{root: ".PHONY: lint\n"}, goCaps, "make-target-missing", model.StatusMeasured},
		{"red required include", map[string]string{root: "include absent.mk\n"}, goCaps, "make-include-missing", model.StatusMeasured},
		{"near optional include", map[string]string{root: "-include absent.mk\n" + buildTree(goCaps, "")[root]}, goCaps, "", model.StatusMeasured},
		{"near conditional include", map[string]string{root: "ifneq ($(wildcard local.mk),)\ninclude local.mk\nendif\n" + buildTree(goCaps, "")[root]}, goCaps, "", model.StatusMeasured},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := measure(t, policyfiles.Makefile(), fixture{
				files: tc.files, capabilities: tc.caps,
			})
			if got.Status.Kind != tc.kind {
				t.Fatalf("status = %+v", got.Status)
			}
			if tc.code == "" {
				if len(got.Findings) != 0 {
					t.Fatalf("unexpected findings: %+v", got.Findings)
				}
			} else if !slices.Contains(codes(got), tc.code) {
				t.Fatalf("codes = %v, want %s", codes(got), tc.code)
			}
		})
	}
}

func TestMakefileModularAndLimitedCases(t *testing.T) {
	t.Parallel()
	const root = "Makefile"
	caps := []string{"go"}
	for _, tc := range []struct {
		name, include string
		limited       bool
	}{
		{"near modular", "include mk/checks.mk\n", false},
		{"red computed include", "include $(PART)\n", true},
		{"near include a rule remakes, a stated limit", "include deps.mk\ndeps.mk:\n\tgo run ./tools/deps > deps.mk\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := measure(t, policyfiles.Makefile(), fixture{
				capabilities: caps,
				files: map[string]string{
					root:           tc.include,
					"mk/checks.mk": buildTree(caps, "")[root],
					"golaw.yml":    "ladder: {pre_commit: false, commit_msg: false, pre_push: false}\n",
				},
			})
			if model.IsOutsideSubset(got.Status) != tc.limited {
				t.Fatalf("status = %+v", got.Status)
			}
			if len(got.Findings) != 0 {
				t.Fatalf("unexpected findings: %+v", got.Findings)
			}
		})
	}
}

func TestMakefileMissingIncludeNamesTheRepair(t *testing.T) {
	t.Parallel()
	got := measure(t, policyfiles.Makefile(), fixture{
		capabilities: []string{"go"},
		files:        map[string]string{"Makefile": "# header\ninclude mk/checks.mk\n"},
	})
	if len(got.Findings) != 1 {
		t.Fatalf("findings = %+v", got.Findings)
	}
	f := got.Findings[0]
	if f.Path != "Makefile" || f.Line != 2 ||
		!strings.Contains(f.Message, "mk/checks.mk") ||
		!strings.Contains(f.Message, "restore") {
		t.Fatalf("repair = %+v", f)
	}
}

func TestMakefileLimitsOnlyUnresolvedRequiredTargets(t *testing.T) {
	t.Parallel()
	caps := []string{"go"}
	for _, tc := range []struct {
		name, omit, extra, construct string
		want                         bool
	}{
		{"near unrelated computed rule", "", "$(BIN): $(SRC)\n", "", false},
		{"red computed missing target", "lint", "$(TARGET):\n", "$(TARGET)", false},
		{"red unrelated prerequisite", "lint", "build: $(SRC)\n", "", true},
		{"red nonmatching pattern", "lint", "%-check:\n", "", true},
		{"red nonmatching computed prefix", "lint", "bin/$(BIN):\n", "", true},
		{"red matching pattern", "lint", "l%:\n", "l%", false},
		{"near conditional rule beside the required targets", "", "ifeq ($(MODE),fast)\nfast: lint\nendif\n", "", false},
		{"red required target only inside a conditional block", "lint", "ifeq ($(MODE),fast)\nlint:\nendif\n", "conditional block", false},
		{"near top-level eval beside the required targets", "", "$(eval $(call RULE,extra))\n", "", false},
		{"red top-level eval where a required target is absent", "lint", "$(eval $(call RULE,lint))\n", "$(eval", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			files := buildTree(caps, tc.omit)
			files["Makefile"] += tc.extra
			got := measure(t, policyfiles.Makefile(), fixture{files: files, capabilities: caps})
			if model.IsOutsideSubset(got.Status) != (tc.construct != "") {
				t.Fatalf("status = %+v", got.Status)
			}
			if tc.construct != "" {
				if len(got.Findings) != 0 || !strings.Contains(got.Status.Reason, tc.construct) ||
					!strings.Contains(got.Status.Reason, "Makefile:2:") {
					t.Fatalf("limit = %+v", got)
				}
			} else if got.Status.Kind != model.StatusMeasured ||
				slices.Contains(codes(got), "make-target-missing") != tc.want {
				t.Fatalf("result = %+v", got)
			}
		})
	}
}

func TestMakefileNamesARecipeTwoFilesDefine(t *testing.T) {
	t.Parallel()
	caps := []string{"go"}
	files := map[string]string{
		"Makefile": "lint:\n\tgolangci-lint run ./internal/...\ninclude golaw.mk\n",
		"golaw.mk": buildTree(caps, "")["Makefile"] + "lint:\n\tgolangci-lint run ./...\n",
	}
	got := measure(t, policyfiles.Makefile(), fixture{files: files, capabilities: caps})
	if got.Status.Kind != model.StatusMeasured || !slices.Equal(codes(got), []string{"make-recipe-overridden"}) {
		t.Fatalf("result = %+v", got)
	}
	if f := got.Findings[0]; f.Path != "golaw.mk" || f.Line != 2 ||
		!strings.Contains(f.Message, "lint") || !strings.Contains(f.Message, "Makefile") {
		t.Fatalf("finding = %+v", f)
	}
	files["Makefile"] = "lint-local:\n\tgolangci-lint run ./internal/...\ninclude golaw.mk\n"
	if near := measure(t, policyfiles.Makefile(), fixture{files: files, capabilities: caps}); near.Status.Kind != model.StatusMeasured || len(near.Findings) != 0 {
		t.Fatalf("a renamed target = %+v", near)
	}
}

// TestMakefileStatesAnotherMakeFileName holds an absent Makefile beside a
// file GNU make reads first or reads on a case-sensitive system to a stated
// limit that names the file and the rename, never to a missing Makefile.
func TestMakefileStatesAnotherMakeFileName(t *testing.T) {
	t.Parallel()
	targets := buildTree([]string{"go"}, "")["Makefile"]
	for _, name := range []string{"GNUmakefile", "makefile"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := measure(t, policyfiles.Makefile(), fixture{files: map[string]string{name: targets}, capabilities: []string{"go"}})
			if !model.IsOutsideSubset(got.Status) || !strings.Contains(got.Status.Reason, name+":1:") || !strings.Contains(got.Status.Reason, "rename it to Makefile") {
				t.Fatalf("%s alone = %+v", name, got)
			}
		})
	}
	got := measure(t, policyfiles.Makefile(), fixture{files: map[string]string{"Makefile": targets, "makefile.md": "notes\n"}, capabilities: []string{"go"}})
	if got.Status.Kind != model.StatusMeasured || len(got.Findings) != 0 {
		t.Fatalf("near a Makefile beside notes = %+v", got)
	}
}
