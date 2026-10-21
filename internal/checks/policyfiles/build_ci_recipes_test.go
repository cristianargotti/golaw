package policyfiles_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestCITargetReadsLiteralMakeRecipesAndDefersRawEvidence(t *testing.T) {
	t.Parallel()
	caps := []string{"go"}
	all := strings.Join(policyfiles.RequiredTargets(caps), " ")
	withoutLint := strings.Replace(all, " lint ", " ", 1)
	for _, tc := range []struct {
		name, body, construct string
		want                  bool
	}{
		{"near recursive make recipe", "ci:\n\t$(MAKE) " + all + "\n", "", false},
		{"near plain make recipe", "ci:\n\t@make " + all + "\n", "", false},
		{"red recipe missing lint", "ci:\n\t$(MAKE) " + withoutLint + "\n", "", true},
		{"near transitive make recipe", "ci:\n\tmake checks\nchecks:\n\t@$(MAKE) " + all + "\n", "", false},
		{"red raw reachable prerequisite", "ci: checks\nchecks: " + withoutLint + " $(MORE)\n", "$(MORE)", false},
		{"red unrelated raw prerequisite", "ci: " + withoutLint + "\ncompile: $(SRC)\n", "", true},
		{"near raw rule beside complete ci", "ci: " + all + "\n$(BIN): $(SRC)\n", "", false},
		{"near opaque recipe beside complete ci", "ci: " + all + "\n\t./scripts/check.sh\n", "", false},
		{"red flags are opaque", "ci:\n\tmake -s " + all + "\n", "make -s", false},
		{"red suppressed failure", "ci:\n\t-$(MAKE) " + all + "\n", "-$(MAKE)", false},
		{"red computed recipe target", "ci:\n\tmake $(CHECKS)\n", "$(CHECKS)", false},
		{"red relevant target definition", "ci: checks\n$(TARGET): " + all + "\n", "$(TARGET)", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			files := buildTree(caps, "")
			files["Makefile"] += tc.body
			got := measure(t, policyfiles.CITarget(), fixture{files: files, capabilities: caps})
			if model.IsOutsideSubset(got.Status) != (tc.construct != "") {
				t.Fatalf("status = %+v", got.Status)
			}
			if tc.construct != "" {
				if len(got.Findings) != 0 || !strings.Contains(got.Status.Reason, tc.construct) ||
					!strings.Contains(got.Status.Reason, "Makefile:") {
					t.Fatalf("limit = %+v", got)
				}
			} else if got.Status.Kind != model.StatusMeasured ||
				slices.Contains(codes(got), "ci-target-missing") != tc.want {
				t.Fatalf("result = %+v", got)
			}
		})
	}
}

func TestCITargetKeepsGraphLimitsRelevant(t *testing.T) {
	t.Parallel()
	caps := []string{"go"}
	all := strings.Join(policyfiles.RequiredTargets(caps), " ")
	withoutLint := strings.Replace(all, " lint ", " ", 1)
	for _, tc := range []struct {
		name, body, construct string
		missing               bool
	}{
		{"absent ci", "", "", true},
		{"unresolved ci", "$(TARGET): " + all + "\n", "$(TARGET)", false},
		{"incomplete cycle", "ci: checks\nchecks: ci\n", "cyclic", false},
		{"complete cycle", "ci: checks " + all + "\nchecks: ci\n", "", false},
		{"repeated prerequisite", "ci: checks" + " checks\nchecks: " + all + "\n", "", false},
		{"raw augmentation", "ci: " + withoutLint + " checks\nchecks:\n$(TARGET): lint\n", "$(TARGET)", false},
		{"carried tool recipe", "ci: " + withoutLint + "\nmod:\n\tgo mod verify\n", "", true},
		{"carried recursive recipe", "ci: " + withoutLint + "\nmod:\n\tmake lint\n", "", false},
		{"order only lint", "ci: " + withoutLint + " | lint\n", "", true},
		{"required include beside complete ci", "include missing.mk\nci: " + all + "\n", "", false},
		{"required include missing lint", "include missing.mk\nci: " + withoutLint + "\n", "missing.mk", false},
		{"required include absent ci", "include missing.mk\n", "missing.mk", false},
		{"optional include", "-include missing.mk\nci: " + all + "\n", "", false},
		{"conditional ci", "ifdef MODE\nci: " + all + "\nendif\n", "conditional block", false},
		{"conditional additions", "ci: " + withoutLint + "\nifdef MODE\nci: lint\nendif\n", "conditional block", false},
		{"complete beside conditional", "ci: " + all + "\nifdef MODE\nci: extra\nendif\n", "", false},
		{"unreachable conditional", "ci: " + withoutLint + "\nifdef MODE\nother: lint\nendif\n", "", true},
		{"conditional include", "ifdef MODE\ninclude missing.mk\nendif\nci: " + withoutLint + "\n", "missing.mk", false},
		{"winning recipe", "ci:\n\tunknown-command\nci:\n\tmake " + withoutLint + "\n", "", true},
		{"blank and comment recipes", "ci:\n\t\n\t# make lint\n", "", true},
		{"multiple shell lines", "ci:\n\tmake checks \\\n\tmake lint\n", "opaque recipe", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			files := buildTree(caps, "")
			files["Makefile"] += tc.body
			got := measure(t, policyfiles.CITarget(), fixture{files: files, capabilities: caps})
			if tc.construct != "" {
				if !model.IsOutsideSubset(got.Status) || len(got.Findings) != 0 || !strings.Contains(got.Status.Reason, tc.construct) {
					t.Fatalf("limit = %+v", got)
				}
				return
			}
			if got.Status.Kind != model.StatusMeasured || slices.Contains(codes(got), "ci-target-missing") != tc.missing {
				t.Fatalf("result = %+v", got)
			}
		})
	}
}

func TestCITargetNamesIncludedSourcesAndRepairs(t *testing.T) {
	t.Parallel()
	caps := []string{"go"}
	files := buildTree(caps, "")
	files["Makefile"] += "include mk/ci.mk\n"
	files["mk/ci.mk"] = "# build checks\nci: $(CHECKS)\n"
	got := measure(t, policyfiles.CITarget(), fixture{files: files, capabilities: caps})
	if !model.IsOutsideSubset(got.Status) || !strings.Contains(got.Status.Reason, "mk/ci.mk:2:") {
		t.Fatalf("include source = %+v", got)
	}
	all := strings.Join(policyfiles.RequiredTargets(caps), " ")
	files["mk/ci.mk"] = "# build checks\nci: " + strings.Replace(all, " lint ", " ", 1) + "\n"
	got = measure(t, policyfiles.CITarget(), fixture{files: files, capabilities: caps})
	if len(got.Findings) != 1 {
		t.Fatalf("missing lint = %+v", got)
	}
	f := got.Findings[0]
	if f.Path != "mk/ci.mk" || f.Line != 2 || !strings.Contains(f.Message, "lint") || !strings.Contains(f.Message, "scheduled") {
		t.Fatalf("repair = %+v", f)
	}
}

func TestCITargetKeepsIndependentJobEvidence(t *testing.T) {
	t.Parallel()
	files := ciTree([]string{"go"}, "vuln")
	files[".github/workflows/demand.yml"] = "on: schedule\njobs:\n  unknown:\n    steps: unknown\n  audit:\n    steps:\n      - run: make vuln\n"
	got := measure(t, policyfiles.CITarget(), fixture{files: files, capabilities: []string{"go"}})
	if got.Status.Kind != model.StatusMeasured || len(got.Findings) != 0 {
		t.Fatalf("independent evidence = %+v", got)
	}
}

func TestCITargetDemandEvidenceCompletesCoverageBesideMissingInclude(t *testing.T) {
	t.Parallel()
	files := ciTree([]string{"go"}, "vuln")
	files["Makefile"] += "include missing.mk\n"
	files[".github/workflows/demand.yml"] = "on: schedule\njobs:\n  audit:\n    steps:\n      - run: make vuln\n"
	got := measure(t, policyfiles.CITarget(), fixture{files: files, capabilities: []string{"go"}})
	if got.Status.Kind != model.StatusMeasured || len(got.Findings) != 0 {
		t.Fatalf("independent demand evidence beside missing include = %+v", got)
	}
}

func TestCITargetReadFailuresKeepTheActualPath(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"Makefile", "mk/checks.mk", ".github/workflows/demand.yml"} {
		for _, failure := range []error{model.ErrTooLarge, model.ErrUnreadable} {
			t.Run(path+"/"+failure.Error(), func(t *testing.T) {
				t.Parallel()
				policy := model.CoreDefaults()
				policy.Capabilities = []string{"go"}
				tree := proof.NewTree(map[string][]byte{
					"Makefile":                     []byte("include mk/checks.mk\n"),
					"mk/checks.mk":                 []byte(ciTree(policy.Capabilities, "vuln")["Makefile"]),
					".github/workflows/demand.yml": []byte("on: schedule\njobs: {}\n"),
				})
				got, err := policyfiles.CITarget().Run(t.Context(), model.Input{
					Policy: &policy, Snapshot: buildReadFailure{Snapshot: tree, path: path, err: failure},
				})
				if err != nil || len(got.Findings) != 0 {
					t.Fatalf("failed CI read = %+v, %v", got, err)
				}
				requireMakefileReadFailure(t, got, path, failure)
			})
		}
	}
}
