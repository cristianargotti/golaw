package setup_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/setup"
)

const routesFix = "apply the lefthook.yml proposal golaw init prints, then run golaw setup"

// TestSetupRefusesALefthookConfigThatRunsNoRung keeps setup from arming
// hooks that run something else: lefthook.yml is the repository's own when
// init only proposed over it, and a rung it does not route to golaw either
// leaves a hook lefthook never creates or arms the repository's commands
// while the receipt says the rungs are armed. The refusal names every
// missing rung and the fix, before any tool is installed.
//
//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestSetupRefusesALefthookConfigThatRunsNoRung(t *testing.T) {
	for name, tc := range map[string]struct {
		config string
		want   []string
	}{
		"no pre-push section": {
			"pre-commit:\n  commands:\n    golaw:\n      run: golaw hook pre-commit\n" +
				"commit-msg:\n  commands:\n    golaw:\n      run: golaw hook commit-msg '{1}'\n",
			[]string{"no pre-push command golaw hook pre-push"},
		},
		"the repository's own commands": {
			"# golaw hook pre-commit runs in CI\npre-commit:\n  commands:\n    hygiene:\n      run: make hygiene\n    vet:\n      glob: \"*.go\"\n      run: go vet ./... # golaw hook pre-commit\n" +
				"commit-msg:\n  commands:\n    lint:\n      run: make lint-commit\n" +
				"pre-push:\n  commands:\n    test:\n      run: make test-staged\n",
			[]string{"no pre-commit command golaw hook pre-commit", "no commit-msg command golaw hook commit-msg", "no pre-push command golaw hook pre-push"},
		},
		"another rung's command": {
			"pre-commit:\n  commands:\n    golaw:\n      run: golaw hook commit-msg '{1}'\n" +
				"commit-msg:\n  commands:\n    golaw:\n      run: golaw hook commit-msg '{1}'\n" +
				"pre-push:\n  commands:\n    golaw:\n      run: golaw hook pre-push '{1}' '{2}'\n",
			[]string{"no pre-commit command golaw hook pre-commit"},
		},
		"no mapping":  {"- golaw hook pre-commit\n", []string{"no pre-commit command golaw hook pre-commit"}},
		"broken yaml": {"pre-commit: [\n", []string{"no pre-commit command golaw hook pre-commit", "no pre-push command golaw hook pre-push"}},
	} {
		t.Run(name, func(t *testing.T) {
			options, deps, probe := setupFixture(t)
			probe.repo.Write("lefthook.yml", tc.config)
			_, err := setup.PlanInstall(t.Context(), options, deps)
			if !errors.Is(err, install.ErrPath) || !strings.Contains(err.Error(), routesFix) {
				t.Fatalf("refusal = %v", err)
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("refusal %q does not name %q", err, want)
				}
			}
			setupSafetyNoInstalls(t, probe)
		})
	}
}

// TestSetupArmsAConfigThatRoutesEveryRung arms a lefthook.yml that runs the
// three rungs beside the repository's own commands, as jobs, through an
// anchor, by the path of the binary or in flow style, one the local file
// completes, and one setup cannot read with certainty: an alias or a
// configuration that extends another one.
//
//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestSetupArmsAConfigThatRoutesEveryRung(t *testing.T) {
	own := "pre-commit:\n  commands:\n    golaw:\n      run: golaw hook pre-commit\n" +
		"commit-msg:\n  commands:\n    golaw:\n      run: golaw hook commit-msg '{1}'\n"
	for name, config := range map[string]string{
		"flow style": "pre-commit: {commands: {golaw: {run: golaw hook pre-commit}}}\n" +
			"commit-msg: {commands: {golaw: {run: \"golaw hook commit-msg {1}\"}}}\n" +
			"pre-push: {commands: {golaw: {run: 'golaw hook pre-push {1} {2}'}}}\n",
		"an alias":  own + "pre-push:\n  commands:\n    golaw:\n      run: *push\n",
		"extends":   "extends:\n  - shared/lefthook.yml\npre-commit:\n  commands:\n    vet:\n      run: go vet ./...\n",
		"the local": own + "pre-push:\n  commands:\n    test:\n      run: make test\n",
		"beside other commands": "pre-commit:\n  parallel: true\n  commands:\n    vet:\n      run: go vet ./...\n    golaw:\n      run: golaw hook pre-commit\n" +
			"commit-msg:\n  commands:\n    golaw:\n      run: golaw hook commit-msg '{1}'\n" +
			"pre-push:\n  commands:\n    golaw:\n      run: golaw hook pre-push '{1}' '{2}'\n      use_stdin: true\n",
		"as jobs, an anchor and a path": "pre-commit:\n  jobs:\n    - name: golaw\n      run: &rung golaw hook pre-commit\n" +
			"commit-msg:\n  jobs:\n    - group:\n        jobs:\n          - run: ./bin/golaw hook commit-msg {1}\n" +
			"pre-push:\n  commands:\n    golaw:\n      run: golaw.exe hook pre-push {1} {2}\n" +
			"post-checkout:\n  commands:\n    echo:\n      run: *rung\n",
	} {
		t.Run(name, func(t *testing.T) {
			options, deps, probe := setupFixture(t)
			probe.repo.Write("lefthook.yml", config)
			if name == "the local" {
				probe.repo.Write("lefthook-local.yml", "pre-push:\n  commands:\n    golaw:\n      run: golaw hook pre-push '{1}' '{2}'\n")
			}
			setupSafetyArm(t, options, deps)
		})
	}
}
