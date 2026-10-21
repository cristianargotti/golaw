package setup

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// TestRoutesAndTheCITargetReadTheRungsAlike holds the two readers of a
// lefthook.yml to one reading of a rung's run value: what setup reads as
// routing the rung to golaw is no finding of policy.ci-target unless a
// segment of it runs a check make ci carries, and such a segment is a
// finding whatever setup answers.
func TestRoutesAndTheCITargetReadTheRungsAlike(t *testing.T) {
	t.Parallel()
	targets := strings.Join(policyfiles.RequiredTargets([]string{"go"}), " ")
	makefile := targets + ":\nci: " + targets + "\n"
	for _, tc := range []struct {
		name, run     string
		routed, heavy bool
	}{
		{"the rung", "golaw hook pre-commit", true, false},
		{"a path to the binary", "./bin/golaw hook pre-commit", true, false},
		{"a quoted Windows path", `"C:\tools\golaw.exe" hook pre-commit`, true, false},
		{"a light command after the rung", "golaw hook pre-commit && commitlint --version", true, false},
		{"a heavy command after the rung", "golaw hook pre-commit && make ci", true, true},
		{"another program alone", "gofmt -l .", false, false},
		{"tests alone", "go test ./...", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := "pre-commit:\n  commands:\n    check:\n      run: '" + strings.ReplaceAll(tc.run, "'", "''") + "'\n" +
				"commit-msg:\n  commands:\n    golaw:\n      run: golaw hook commit-msg {1}\n" +
				"pre-push:\n  commands:\n    golaw:\n      run: golaw hook pre-push\n"
			if routed := requireRoutes([]byte(body)) == nil; routed != tc.routed {
				t.Fatalf("setup reads the rung as routed = %v, want %v", routed, tc.routed)
			}
			policy := model.CoreDefaults()
			policy.Capabilities = []string{"go"}
			tree := proof.NewTree(map[string][]byte{"Makefile": []byte(makefile), "lefthook.yml": []byte(body)})
			result, err := policyfiles.CITarget().Run(t.Context(), model.Input{
				Mode: model.ModeScan, Policy: &policy, Snapshot: tree, Targets: tree.Files(),
			})
			if err != nil || result.Status.Kind != model.StatusMeasured {
				t.Fatalf("policy.ci-target = %+v, %v", result.Status, err)
			}
			heavy := slices.ContainsFunc(result.Findings, func(f model.Finding) bool { return f.Code == "hook-command-not-binary" })
			if heavy != tc.heavy {
				t.Fatalf("the pipeline reads the run value as heavier = %v, want %v: %+v", heavy, tc.heavy, result.Findings)
			}
		})
	}
}
