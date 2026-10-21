package policyfiles_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

func ciTree(capabilities []string, omit string) map[string]string {
	files := buildTree(capabilities, "")
	targets := policyfiles.RequiredTargets(capabilities)
	targets = slices.DeleteFunc(targets, func(value string) bool {
		return value == omit
	})
	files["Makefile"] += "ci: " + strings.Join(targets, " ") + "\n"
	return files
}

func TestCITargetCarriesChecksOrMeasuredDemandEvidence(t *testing.T) {
	t.Parallel()
	caps := []string{"go"}
	const action = ".github/workflows/demand.yml"
	for _, tc := range []struct {
		name, omit, lane string
		want             bool
		limited          bool
	}{
		{"red removed check", "lint", "", true, false},
		{"near complete", "", "", false, false},
		{"near scheduled", "vuln", "on:\n  schedule:\n    - cron: '0 0 * * *'\njobs:\n  audit:\n    steps:\n      - run: make vuln\n", false, false},
		{"near manually dispatched", "vuln", "on: workflow_dispatch\njobs:\n  audit:\n    steps:\n      - run: make vuln\n", false, false},
		{"red push only", "vuln", "on: push\njobs:\n  audit:\n    steps:\n      - run: make vuln\n", true, false},
		{"red text is not execution", "vuln", "on: workflow_dispatch\njobs:\n  audit:\n    steps:\n      - run: echo make vuln\n", true, false},
		{"red dynamic invocation", "vuln", "on: workflow_dispatch\njobs:\n  audit:\n    steps:\n      - run: make $TARGET\n", false, true},
		{"red conditional evidence", "vuln", "on: workflow_dispatch\njobs:\n  audit:\n    if: false\n    steps:\n      - run: make vuln\n", true, false},
		{"red scheduled reusable job", "vuln", "on: schedule\njobs:\n  audit:\n    uses: owner/repo/file@v1\n", false, true},
		{"near complete with reusable job", "", "on: schedule\njobs:\n  audit:\n    uses: owner/repo/file@v1\n", false, false},
		{"red push reusable job", "vuln", "on: push\njobs:\n  audit:\n    uses: owner/repo/file@v1\n", true, false},
		{"near reusable beside measured evidence", "vuln", "on: workflow_dispatch\njobs:\n  remote:\n    uses: owner/repo/file@v1\n  audit:\n    steps:\n      - run: make vuln\n", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			files := ciTree(caps, tc.omit)
			if tc.lane != "" {
				files[action] = tc.lane
			}
			got := measure(t, policyfiles.CITarget(), fixture{
				files: files, capabilities: caps,
			})
			requireCITargetEvidence(t, got, tc.name, action, tc.limited, tc.want)
		})
	}
}

func requireCITargetEvidence(t *testing.T, got model.Result, name, action string, limited, want bool) {
	t.Helper()
	if model.IsOutsideSubset(got.Status) != limited {
		t.Fatalf("status = %+v", got.Status)
	}
	if name == "red scheduled reusable job" &&
		(!strings.Contains(got.Status.Reason, action+":") || !strings.Contains(got.Status.Reason, "audit")) {
		t.Fatalf("reusable job limit = %+v", got.Status)
	}
	if limited {
		if len(got.Findings) != 0 {
			t.Fatalf("limit produced findings: %+v", got.Findings)
		}
		return
	}
	if got.Status.Kind != model.StatusMeasured {
		t.Fatalf("unmeasured: %+v", got.Status)
	}
	if slices.Contains(codes(got), "ci-target-missing") != want {
		t.Fatalf("findings = %+v", got.Findings)
	}
}

func TestCITargetAcceptsTransitivePrerequisites(t *testing.T) {
	t.Parallel()
	caps := []string{"go"}
	files := buildTree(caps, "")
	files["Makefile"] += "ci: checks\nchecks: " +
		strings.Join(policyfiles.RequiredTargets(caps), " ") + "\n"
	got := measure(t, policyfiles.CITarget(), fixture{
		files: files, capabilities: caps,
	})
	if got.Status.Kind != model.StatusMeasured || len(got.Findings) != 0 {
		t.Fatalf("transitive result = %+v", got)
	}
}

func TestCITargetLeavesAnAbsentMakefileToPolicyMakefile(t *testing.T) {
	t.Parallel()
	got := measure(t, policyfiles.CITarget(), fixture{capabilities: []string{"go"}})
	if got.Status.Kind != model.StatusMeasured || len(got.Findings) != 0 {
		t.Fatalf("absent Makefile = %+v", got)
	}
}

func TestCITargetNamesUnknownExecutionWithoutFinding(t *testing.T) {
	t.Parallel()
	files := buildTree([]string{"go"}, "")
	files["Makefile"] += "ci:\n\t./scripts/check.sh\n"
	got := measure(t, policyfiles.CITarget(), fixture{
		files: files, capabilities: []string{"go"},
	})
	if !model.IsOutsideSubset(got.Status) || len(got.Findings) != 0 {
		t.Fatalf("unknown execution = %+v", got)
	}
}

func TestCITargetReadsDemandExecutionControls(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, root, job, step, run string
		limited, missing           bool
	}{
		{"plain multiline", "", "", "", "|\n          echo checking\n          # make lint\n          make vuln", false, false},
		{"step disabled", "", "", "        if: false\n", "make vuln", false, true},
		{"job condition", "", "    if: unknown\n", "", "make vuln", true, false},
		{"step condition", "", "", "        if: unknown\n", "make vuln", true, false},
		{"job suppresses failure", "", "    continue-on-error: true\n", "", "make vuln", true, false},
		{"step suppresses failure", "", "", "        continue-on-error: true\n", "make vuln", true, false},
		{"safe controls", "", "    if: true\n", "        continue-on-error: false\n", "make vuln", false, false},
		{"custom shell", "", "", "        shell: bash {0}\n", "|\n          make vuln\n          true", true, false},
		{"pwsh masks native failure", "", "", "        shell: pwsh\n", "|\n          make vuln\n          make lint", true, false},
		{"powershell masks native failure", "", "", "        shell: powershell\n", "|\n          make vuln\n          make lint", true, false},
		{"cmd masks native failure", "", "", "        shell: cmd\n", "|\n          make vuln\n          make lint", true, false},
		{"windows default shell", "", "    runs-on: windows-latest\n", "", "|\n          make vuln\n          make lint", true, false},
		{"windows label default shell", "", "    runs-on: [self-hosted, Windows]\n", "", "make vuln", true, false},
		{"windows mapping default shell", "", "    runs-on:\n      labels: windows-2022\n", "", "make vuln", true, false},
		{"windows explicit safe shell", "", "    runs-on: windows-latest\n", "        shell: bash\n", "make vuln", false, false},
		{"nonwindows default shell", "", "    runs-on: ubuntu-latest\n", "", "make vuln", false, false},
		{"root custom shell", "defaults:\n  run:\n    shell: bash {0}\n", "", "", "make vuln", true, false},
		{"shell override", "defaults:\n  run:\n    shell: bash {0}\n", "", "        shell: bash\n", "make vuln", false, false},
		{"evaluated failure mode", "", "", "", "|\n          eval 'set +e'\n          make vuln\n          true", true, false},
		{"sourced failure mode", "", "", "", "|\n          . ./options.sh\n          make vuln\n          true", true, false},
		{"root directory", "defaults:\n  run:\n    working-directory: src\n", "", "", "make vuln", true, false},
		{"job directory", "", "    defaults:\n      run:\n        working-directory: src\n", "", "make vuln", true, false},
		{"step directory", "", "", "        working-directory: src\n", "make vuln", true, false},
		{"root override", "defaults:\n  run:\n    working-directory: src\n", "", "        working-directory: ./\n", "make vuln", false, false},
		{"invalid defaults", "defaults: other\n", "", "", "make vuln", true, false},
		{"duplicate event", "on: push\n", "", "", "make vuln", true, false},
		{"duplicate condition", "", "    if: true\n    if: false\n", "", "make vuln", true, false},
		{"duplicate directory", "defaults:\n  run:\n    working-directory: .\n    working-directory: src\n", "", "", "make vuln", true, false},
		{"compound", "", "", "", "make vuln || true", true, false},
		{"negated", "", "", "", "'! make vuln'", true, false},
		{"shell conditional", "", "", "", "|\n          if false; then\n            make vuln\n          fi", true, false},
		{"failure mode", "", "", "", "|\n          set +e\n          make vuln", true, false},
		{"directory command", "", "", "", "|\n          cd src\n          make vuln", true, false},
		{"quoted multiline", "", "", "", "|\n          echo '\n          make vuln\n          '", true, false},
		{"inline text", "", "", "", `echo "make vuln"`, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			files := ciTree([]string{"go"}, "vuln")
			files[".github/workflows/demand.yml"] = "on: workflow_dispatch\n" + tc.root +
				"jobs:\n  audit:\n" + tc.job + "    steps:\n      - run: " + tc.run + "\n" + tc.step
			got := measure(t, policyfiles.CITarget(), fixture{files: files, capabilities: []string{"go"}})
			if model.IsOutsideSubset(got.Status) != tc.limited ||
				slices.Contains(codes(got), "ci-target-missing") != tc.missing || tc.limited && len(got.Findings) != 0 {
				t.Fatalf("execution evidence = %+v", got)
			}
		})
	}
}

func TestCITargetKeepsDemandEvidenceAcrossFiles(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, trigger string
		complete      bool
	}{
		{"push-only unsupported job", "push", false},
		{"unsupported beside measured file", "schedule", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			files := ciTree([]string{"go"}, "vuln")
			files[".github/workflows/a.yml"] = "on: " + tc.trigger + "\njobs:\n  audit:\n    steps: unknown\n"
			if tc.complete {
				files[".github/workflows/b.yml"] = "on: schedule\njobs:\n  audit:\n    steps:\n      - run: make vuln\n"
			}
			got := measure(t, policyfiles.CITarget(), fixture{files: files, capabilities: []string{"go"}})
			if got.Status.Kind != model.StatusMeasured || slices.Contains(codes(got), "ci-target-missing") == tc.complete {
				t.Fatalf("separate files = %+v", got)
			}
		})
	}
}
