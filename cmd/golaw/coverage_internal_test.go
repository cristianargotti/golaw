package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/coverage"
	"github.com/cristianargotti/golaw/internal/gittest"
)

const commandModule = "example.test/coverage"

func coverageCommandFixture(t *testing.T, floor int) (testApp, *gittest.Repo) {
	t.Helper()
	repo := gittest.New(t)
	policyText := "golaw: 1\nkit: {version: " + testVersion + "}\n"
	if floor == 90 {
		policyText += "checks:\n  coverage: {total: 90, package: 90, file: 90}\n"
	}
	repo.Commit("test: seed coverage", map[string]string{
		"go.mod":    "module " + commandModule + "\n\ngo 1.27\n",
		"golaw.yml": policyText,
		"value.go": "package coverage\n\nfunc Value() int {\n" +
			"\tn := 0\n" + strings.Repeat("\tn++\n", 18) + "\treturn n\n}\n",
	})
	repo.Write("profile.cov", "mode: atomic\n"+
		commandModule+"/value.go:4.2,19.5 16 1\n"+
		commandModule+"/value.go:20.2,23.10 4 0\n")
	return bindApp(t), repo
}

func TestCoverageCommandCodesAndPrecedence(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		floor   int
		flags   []string
		missing bool
		code    int
	}{
		{"near-defaults", 80, nil, false, exitClean},
		{"red-policy-floor", 90, nil, false, exitFindings},
		{"red-flags-never-loosen-the-policy", 90, []string{
			"--total", "80", "--package", "80", "--file", "80",
		}, false, exitFindings},
		{"red-flag-tightens", 80, []string{"--total", "85"}, false, exitFindings},
		{"near-flag-at-the-policy", 80, []string{"--total", "80"}, false, exitClean},
		{"red-missing-profile", 80, nil, true, exitFailure},
		{"red-invalid-percentage", 80, []string{"--total", "101"}, false, exitFailure},
		{"red-invalid-minimum", 80, []string{"--min-statements", "-1"}, false, exitFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ta, repo := coverageCommandFixture(t, tc.floor)
			name := "profile.cov"
			if tc.missing {
				name = "absent.cov"
			}
			args := make([]string, 0, 4+len(tc.flags))
			args = append(args, "coverage", "--input", filepath.Join(repo.Root, name))
			args = append(args, tc.flags...)
			args = append(args, repo.Root)
			if got := ta.run(t.Context(), args); got != tc.code {
				t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
			}
			if tc.code == exitFailure && ta.stderr.Len() == 0 {
				t.Fatal("failure has no diagnostic")
			}
		})
	}
}

func TestCoverageUsesHeadPolicyAndArmsNothing(t *testing.T) {
	t.Parallel()
	ta, repo := coverageCommandFixture(t, 90)
	repo.Write("golaw.yml", "golaw: 1\nkit: {version: "+testVersion+"}\n")
	args := []string{
		"coverage", "--input", filepath.Join(repo.Root, "profile.cov"), repo.Root,
	}
	if got := ta.run(t.Context(), args); got != exitFindings {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	if !strings.Contains(ta.stdout.String(), "below the floor") {
		t.Fatalf("stdout = %q", ta.stdout.String())
	}
	for _, name := range []string{"pre-commit", "commit-msg", "pre-push"} {
		_, err := os.Stat(filepath.Join(repo.GitDir(), "hooks", name))
		if !os.IsNotExist(err) {
			t.Fatalf("unexpected installed hook %s: %v", name, err)
		}
	}
}

func TestCoveragePrintsIndependentUniverses(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		plain bool
		want  string
	}{
		{"near-plain-absent", false, "profile --plain-input: not given"},
		{"near-both", true, "profile --plain-input"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ta, repo := coverageCommandFixture(t, 80)
			args := []string{
				"coverage", "--input", filepath.Join(repo.Root, "profile.cov"),
			}
			if tc.plain {
				repo.Write("plain.cov", "mode: set\n"+
					commandModule+"/value.go:4.2,23.10 20 1\n")
				args = append(args, "--plain-input", filepath.Join(repo.Root, "plain.cov"))
			}
			args = append(args, repo.Root)
			if got := ta.run(t.Context(), args); got != exitClean {
				t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
			}
			text := ta.stdout.String()
			if !strings.Contains(text, tc.want) ||
				!strings.Contains(text, "profile --input") ||
				!strings.Contains(text, "total: 80.0% (16/20)") {
				t.Fatalf("stdout = %q", text)
			}
			if tc.plain && !strings.Contains(text, "total: 100.0% (20/20)") {
				t.Fatalf("plain report = %q", text)
			}
		})
	}
}

func TestCoverageRequiresInputAndDoesNotInventPlainCoverage(t *testing.T) {
	t.Parallel()
	ta := bindApp(t)
	if got := ta.run(t.Context(), []string{"coverage"}); got != exitFailure {
		t.Fatalf("exit = %d", got)
	}
	report := coverage.Report{
		Total: coverage.Measure{Name: "total", Statements: 20, Covered: 16},
	}
	text := strings.Join(coverageLines(report, nil), "\n")
	if strings.Count(text, "total:") != 1 ||
		!strings.Contains(text, "profile --plain-input: not given") {
		t.Fatalf("lines = %q", text)
	}
}

func TestCoverageMinimumOnlyTightens(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		policy string
		flag   string
		code   int
	}{
		{"near-policy-minimum", "20", "", exitClean},
		{"red-lower-minimum", "20", "10", exitFindings},
		{"red-zero-minimum", "20", "0", exitFindings},
		{"red-policy-tightened", "10", "", exitFindings},
		{"red-flag-cannot-loosen", "10", "20", exitFindings},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ta, repo := coverageCommandFixture(t, 80)
			repo.Commit("test: set coverage minimum", map[string]string{
				"golaw.yml": "golaw: 1\nkit: {version: " + testVersion + "}\n" +
					"checks:\n  coverage: {min_statements: " + tc.policy + "}\n",
				"small.go": "package coverage\n\nfunc Small() int {\n" +
					"\tn := 0\n" + strings.Repeat("\tn++\n", 8) + "\treturn n\n}\n",
			})
			repo.Write("profile.cov", "mode: atomic\n"+
				commandModule+"/value.go:4.2,23.10 20 1\n"+
				commandModule+"/small.go:4.2,8.5 5 1\n"+
				commandModule+"/small.go:9.2,13.10 5 0\n")
			args := []string{"coverage", "--input", filepath.Join(repo.Root, "profile.cov")}
			if tc.flag != "" {
				args = append(args, "--min-statements", tc.flag)
			}
			args = append(args, repo.Root)
			if got := ta.run(t.Context(), args); got != tc.code {
				t.Fatalf("exit = %d, stdout = %q, stderr = %q", got, ta.stdout.String(), ta.stderr.String())
			}
			if tc.code == exitFindings && !strings.Contains(ta.stdout.String(), "below the floor: small.go") {
				t.Fatalf("small source was not held to the floor: %q", ta.stdout.String())
			}
		})
	}
}

func TestCoverageInvalidFlagsNameTheirCause(t *testing.T) {
	t.Parallel()
	_, repo := coverageCommandFixture(t, 80)
	values := map[string][]string{
		"total":          {"79", "101", "NaN", "+Inf", "-Inf"},
		"package":        {"79", "101", "NaN", "+Inf", "-Inf"},
		"file":           {"79", "101", "NaN", "+Inf", "-Inf"},
		"min-statements": {"-1", "21"},
	}
	for flag, invalid := range values {
		for _, value := range invalid {
			t.Run(flag+"/"+value, func(t *testing.T) {
				t.Parallel()
				ta := bindApp(t)
				args := []string{
					"coverage", "--input", filepath.Join(repo.Root, "profile.cov"),
					"--" + flag, value, repo.Root,
				}
				if got := ta.run(t.Context(), args); got != exitFailure {
					t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
				}
				if !strings.Contains(ta.stderr.String(), "--"+flag) {
					t.Fatalf("diagnostic does not name --%s: %q", flag, ta.stderr.String())
				}
			})
		}
	}
}

func TestCoveragePlainInputControlsTheOutcome(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		profile string
		code    int
	}{
		{"red-floor", "mode: set\n" + commandModule + "/value.go:4.2,23.10 20 0\n", exitFindings},
		{"red-missing", "", exitFailure},
		{"red-malformed", "invalid\n", exitFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ta, repo := coverageCommandFixture(t, 80)
			if tc.profile != "" {
				repo.Write("plain.cov", tc.profile)
			}
			args := []string{
				"coverage", "--input", filepath.Join(repo.Root, "profile.cov"),
				"--plain-input", filepath.Join(repo.Root, "plain.cov"), repo.Root,
			}
			if got := ta.run(t.Context(), args); got != tc.code {
				t.Fatalf("exit = %d, stdout = %q, stderr = %q", got, ta.stdout.String(), ta.stderr.String())
			}
			assertPlainCoverageOutput(t, ta, tc.code)
		})
	}
}

func assertPlainCoverageOutput(t *testing.T, ta testApp, code int) {
	t.Helper()
	if code == exitFailure && ta.stderr.Len() == 0 {
		t.Fatal("plain input failure has no diagnostic")
	}
	if code == exitFindings && (!strings.Contains(ta.stdout.String(), "profile --plain-input") ||
		!strings.Contains(ta.stdout.String(), "total: 0.0% (0/20)")) {
		t.Fatalf("plain report = %q", ta.stdout.String())
	}
}
