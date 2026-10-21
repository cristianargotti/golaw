package main

import (
	"path/filepath"
	"testing"
)

// coveragePolicy is a policy of the test version with a capability line and
// floors the fixture's profile breaks.
func coveragePolicy(capabilities string) string {
	return "golaw: 1\nkit: {version: " + testVersion + "}\n" + capabilities + "checks:\n  coverage: {total: 90, package: 90, file: 90}\n"
}

func TestCoverageIsNotApplicableWithTheGoCapabilityOff(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		capabilities string
		profile      string
		code         int
		stdout       string
	}{
		"go off, no profile on disk": {"capabilities: [image]\n", "absent.cov", exitClean, "coverage: not applicable, the go capability is off\n"},
		"go off, a failing profile":  {"capabilities: []\n", "profile.cov", exitClean, "coverage: not applicable, the go capability is off\n"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ta, repo := coverageCommandFixture(t, 80)
			repo.Commit("test: set the capabilities", map[string]string{"golaw.yml": coveragePolicy(tc.capabilities)})
			args := []string{"coverage", "--input", filepath.Join(repo.Root, tc.profile), repo.Root}
			if got := ta.run(t.Context(), args); got != tc.code || ta.stdout.String() != tc.stdout {
				t.Fatalf("exit = %d, stdout = %q, stderr = %q", got, ta.stdout.String(), ta.stderr.String())
			}
		})
	}
}

func TestCoverageJudgesTheProfileWhileGoHolds(t *testing.T) {
	t.Parallel()
	ta, repo := coverageCommandFixture(t, 80)
	repo.Commit("test: set the capabilities", map[string]string{"golaw.yml": coveragePolicy("capabilities: [go, image]\n")})
	args := []string{"coverage", "--input", filepath.Join(repo.Root, "profile.cov"), repo.Root}
	if got := ta.run(t.Context(), args); got != exitFindings {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", got, ta.stdout.String(), ta.stderr.String())
	}
	usage := bindApp(t)
	if got := usage.run(t.Context(), []string{"coverage", repo.Root}); got != exitFailure {
		t.Fatalf("no --input: exit = %d", got)
	}
}
