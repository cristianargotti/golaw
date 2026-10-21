package policyfiles_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestCITargetKeepsLocalRungsOptionalAndLight(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       bool
	}{
		{"near absent", "", false},
		{"near empty", "min_version: 2.1.0\n", false},
		{"near unrelated git hook", "post-checkout:\n  commands:\n    lfs:\n      run: git lfs post-checkout\n", false},
		{"near one optional rung", "pre-commit:\n  commands:\n    golaw:\n      run: golaw hook pre-commit\n", false},
		{"near quoted arguments", "pre-push:\n  commands:\n    golaw:\n      run: golaw hook pre-push '{1}' '{2}'\n      use_stdin: true\n", false},
		{"near a light command a team adds", "commit-msg:\n  commands:\n    lint:\n      run: commitlint --edit {1}\n", false},
		{"near script entry", "pre-commit:\n  scripts:\n    check.sh:\n      runner: sh\n", false},
		{"near compound with a light command", "pre-commit:\n  commands:\n    check:\n      run: golaw hook pre-commit && custom-check\n", false},
		{"red full CI locally", "pre-push:\n  commands:\n    heavy:\n      run: make ci\n", true},
		{"red tests locally", "pre-push:\n  commands:\n    heavy:\n      run: go test -race ./...\n", true},
		{"red heavy segment of a compound", "pre-commit:\n  commands:\n    check:\n      run: golaw hook pre-commit && make lint\n", true},
		{"red whole-tree lint", "pre-commit:\n  commands:\n    lint:\n      run: golangci-lint run ./...\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			files := ciTree([]string{"go"}, "")
			if tc.body != "" {
				files["lefthook.yml"] = tc.body
			}
			got := measure(t, policyfiles.CITarget(), fixture{
				files: files, capabilities: []string{"go"},
			})
			if got.Status.Kind != model.StatusMeasured {
				t.Fatalf("a lefthook.yml is never a limit of this check: %+v", got.Status)
			}
			if slices.Contains(codes(got), "hook-command-not-binary") != tc.want {
				t.Fatalf("findings = %+v", got.Findings)
			}
		})
	}
}

func TestCITargetRecognizesOnlyLiteralHeavyHookSegments(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		run  string
		want bool
	}{
		{`"C:\tools\go.exe" test ./...`, true},
		{`'/tools/go' vet ./...`, true},
		{"go build ./...", true},
		{"gremlins unleash", true},
		{"custom || make lint", true},
		{"custom; golangci-lint run", true},
		{"custom | go vet ./...", true},
		{"make 'ci'", true},
		{"make ci # local invocation", true},
		{"make", false},
		{"go version", false},
		{"golangci-lint version", false},
		{"gremlins version", false},
		{"make --version", false},
		{"make -C other", false},
		{"make TARGET=ci", false},
		{"$TOOL test ./...", false},
		{"make $TARGET", false},
		{"make \"$TARGET\"", false},
		{"env go test ./...", false},
		{"go 'test", false},
		{`echo "ready && make ci"`, false},
		{`echo 'ready; go test ./...'`, false},
		{`echo $(echo ready; make ci; echo done)`, false},
		{"echo `make ci`", false},
		{"echo # ready; make ci", false},
		{"custom & make ci", false},
		{`"" go test`, false},
		{"go", false},
		{"make ''", false},
	} {
		t.Run(tc.run, func(t *testing.T) {
			t.Parallel()
			files := ciTree([]string{"go"}, "")
			files["lefthook.yml"] = "pre-commit:\n  commands:\n    check:\n      run: '" + strings.ReplaceAll(tc.run, "'", "''") + "'\n"
			got := measure(t, policyfiles.CITarget(), fixture{files: files, capabilities: []string{"go"}})
			if got.Status.Kind != model.StatusMeasured || slices.Contains(codes(got), "hook-command-not-binary") != tc.want {
				t.Fatalf("literal hook result = %+v", got)
			}
			for _, found := range got.Findings {
				if found.Path != "lefthook.yml" || found.Line != 3 || !strings.Contains(found.Message, "check") ||
					!strings.Contains(found.Message, "make ci") || !strings.Contains(found.Message, "golaw hook pre-commit") {
					t.Fatalf("hook repair = %+v", found)
				}
			}
		})
	}
}

func TestCITargetDoesNotTurnHookReaderLimitsIntoFindings(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		"[",
		"extends: ../hooks.yml\n",
		"pre-commit:\n  commands:\n    check:\n      run: [make, ci]\n",
		"post-checkout:\n  commands:\n    check:\n      run: make ci\n",
	} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()
			files := ciTree([]string{"go"}, "")
			files["lefthook.yml"] = body
			got := measure(t, policyfiles.CITarget(), fixture{files: files, capabilities: []string{"go"}})
			if got.Status.Kind != model.StatusMeasured || len(got.Findings) != 0 {
				t.Fatalf("optional hook result = %+v", got)
			}
		})
	}
}

func TestCITargetKeepsFailedHookReadsUnmeasured(t *testing.T) {
	t.Parallel()
	for _, failure := range []error{model.ErrTooLarge, model.ErrUnreadable} {
		t.Run(failure.Error(), func(t *testing.T) {
			t.Parallel()
			policy := model.CoreDefaults()
			policy.Capabilities = []string{"go"}
			tree := proof.NewTree(map[string][]byte{"Makefile": []byte(ciTree(policy.Capabilities, "")["Makefile"])})
			got, err := policyfiles.CITarget().Run(t.Context(), model.Input{
				Policy: &policy, Snapshot: buildReadFailure{Snapshot: tree, path: "lefthook.yml", err: failure},
			})
			if err != nil || len(got.Findings) != 0 {
				t.Fatalf("failed hook read = %+v, %v", got, err)
			}
			requireMakefileReadFailure(t, got, "lefthook.yml", failure)
		})
	}
}

func TestCITargetLimitsOnlyRelevantDemandSteps(t *testing.T) {
	t.Parallel()
	const file = ".github/workflows/demand.yml"
	for _, tc := range []struct{ name, job, step, run, reason string }{
		{"unrelated redirect", "", "", `echo "GOTOOLCHAIN=auto" >> "$GITHUB_ENV"`, ""},
		{"unrelated job condition", "    if: always()\n", "", "go install x", ""},
		{"unrelated Windows runner", "    runs-on: windows-latest\n", "", "echo checking", ""},
		{"unrelated step condition", "", "        if: always()\n", "echo checking", ""},
		{"unrelated directory", "", "        working-directory: src\n", "go install x", ""},
		{"unrelated defaults", "    defaults: {run: {shell: unknown, working-directory: src}}\n", "", "echo checking", ""},
		{"environment prefix", "", "", "GOFLAGS=-mod=mod make vuln", "6: Actions run: GOFLAGS=-mod=mod make vuln"},
		{"failure suppression prefix", "", "", "MAKEFLAGS=-i make vuln", "6: Actions run: MAKEFLAGS=-i make vuln"},
		{"multiple assignments", "", "", "GOFLAGS= MAKEFLAGS=-i make vuln", "6: Actions run: GOFLAGS= MAKEFLAGS=-i make vuln"},
		{"quoted environment", "", "", `GOFLAGS="-mod=mod -race" make vuln`, `6: Actions run: GOFLAGS="-mod=mod -race" make vuln`},
		{"single quoted environment", "", "", "FLAGS='x y' make vuln", "6: Actions run: FLAGS='x y' make vuln"},
		{"assigned echo", "", "", "FLAGS='x y' echo make vuln", ""},
		{"echo assignments", "", "", "echo FLAGS=x make vuln", ""},
		{"quoted word condition", "    if: always()\n", "", "echo 'make' vuln", "4: Actions if: always()"},
		{"operator separators", "", "", "cd src&&make vuln", "6: Actions run: cd src&&make vuln"},
		{"suppressing operator", "", "", "make vuln||true", "6: Actions run: make vuln||true"},
		{"literal block position", "", "", "|\n          echo ready\n          make vuln || true", "8: Actions run: make vuln || true"},
		{"folded block position", "", "", ">\n          GOFLAGS=-mod=mod make vuln", "7: Actions run: GOFLAGS=-mod=mod make vuln"},
		{"whole relevant step", "", "", "|\n          set +e\n          make vuln", "7: Actions run: set +e"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			files := ciTree([]string{"go"}, "vuln")
			files[file] = "on: schedule\njobs:\n  audit:\n" + tc.job + "    steps:\n      - name: evidence\n" + tc.step + "        run: " + tc.run + "\n"
			got := measure(t, policyfiles.CITarget(), fixture{files: files, capabilities: []string{"go"}})
			if tc.reason != "" {
				if !model.IsOutsideSubset(got.Status) || len(got.Findings) != 0 || !strings.Contains(got.Status.Reason, file+":"+tc.reason) {
					t.Fatalf("relevant limit = %+v", got)
				}
				return
			}
			if got.Status.Kind != model.StatusMeasured || !slices.Equal(codes(got), []string{"ci-target-missing"}) {
				t.Fatalf("missing demand evidence = %+v", got)
			}
		})
	}
}
