package main

import (
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

const newCodeModule = "example.test/sample"

// newCodeBase is the source both periods start from.
const newCodeBase = "package sample\n\n// Value starts the count.\nfunc Value() int {\n\tn := 0\n\treturn n\n}\n"

// newCodeHead appends five one-line functions to the base: their return
// lines, 10, 14, 18, 22 and 26, are the new lines to cover.
func newCodeHead() string {
	names := []string{"One", "Two", "Three", "Four", "Five"}
	parts := make([]string, 0, len(names)+1)
	parts = append(parts, newCodeBase)
	for i, name := range names {
		parts = append(parts, "\nfunc "+name+"() int {\n\treturn "+strconv.Itoa(i+1)+"\n}\n")
	}
	return strings.Join(parts, "")
}

// newCodeProfile is the pipeline report of newCodeHead with Value and the
// first covered functions run, plus extra block lines.
func newCodeProfile(covered int, extra ...string) string {
	lines := []string{"mode: set"}
	for i, block := range []string{"4.18,7.2", "9.16,11.2", "13.16,15.2", "17.18,19.2", "21.17,23.2", "25.17,27.2"} {
		hit := "0"
		if i <= covered {
			hit = "1"
		}
		lines = append(lines, newCodeModule+"/value.go:"+block+" 1 "+hit)
	}
	return strings.Join(append(lines, extra...), "\n") + "\n"
}

type newCodeCase struct {
	policy   string            // appended to the golaw.yml of both commits
	extra    map[string]string // files the change commit adds or replaces
	profiled bool              // pin and install the synthetic surface profile
}

type newCodeRepo struct {
	ta   testApp
	repo *gittest.Repo
}

// newCodeFixture is a module whose base commit holds Value, with the
// branch base and the tag v1.0.0 on it, and whose HEAD appends the five
// functions; its binding sits under a temporary home.
func newCodeFixture(t *testing.T, c newCodeCase) newCodeRepo {
	t.Helper()
	ta := bindApp(t)
	policyText := "golaw: 1\nkit: {version: " + testVersion + "}\ncapabilities: [go]\n" + c.policy
	if c.profiled {
		pinned, err := bootstrap.SetProfile([]byte(policyText), profiletest.SurfaceRef(t))
		if err != nil {
			t.Fatal(err)
		}
		policyText = string(pinned)
		dirs, err := ta.bind().dirs()
		if err != nil {
			t.Fatal(err)
		}
		profiletest.SurfaceInstall(t, dirs.Config)
	}
	repo := gittest.New(t)
	base := repo.Commit("feat: the base", map[string]string{
		"go.mod": "module " + newCodeModule + "\n\ngo 1.27\n", "golaw.yml": policyText, "value.go": newCodeBase,
	})
	repo.Git("branch", "base")
	repo.Git("tag", "v1.0.0", base)
	head := map[string]string{"value.go": newCodeHead()}
	maps.Copy(head, c.extra)
	repo.Commit("feat: five functions", head)
	return newCodeRepo{ta: ta, repo: repo}
}

func chtimes(t *testing.T, path string, at time.Time) {
	t.Helper()
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
}

// write places the report and sets the times staleness reads: every Go
// source at one instant, the report an hour after it when fresh and an
// hour before it when stale.
func (f newCodeRepo) write(t *testing.T, name, profile string, fresh bool) string {
	t.Helper()
	f.repo.Write(name, profile)
	sources := time.Date(2026, time.October, 27, 9, 0, 0, 0, time.UTC)
	reportTime := sources.Add(time.Hour)
	if !fresh {
		reportTime = sources.Add(-time.Hour)
	}
	walkErr := filepath.WalkDir(f.repo.Root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".go") {
			chtimes(t, path, sources)
		}
		return nil
	})
	if walkErr != nil {
		t.Fatal(walkErr)
	}
	report := filepath.Join(f.repo.Root, filepath.FromSlash(name))
	chtimes(t, report, reportTime)
	return report
}

func (f newCodeRepo) run(t *testing.T, args ...string) (int, string) {
	t.Helper()
	full := append([]string{"coverage"}, args...)
	code := f.ta.run(t.Context(), append(full, f.repo.Root))
	return code, f.ta.stdout.String() + f.ta.stderr.String()
}

func TestCoverageNewCodeVerdicts(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		policy  string
		covered int
		fresh   bool
		flags   []string
		code    int
		want    string
	}{
		{"red-below-the-sonar-way-threshold", "", 3, true, nil, exitFindings, "value.go: 3/5 covered, uncovered lines 22, 26"},
		{"near-at-the-threshold", "", 4, true, nil, exitClean, "new code: 80.0% (4/5 new lines covered), threshold 80%"},
		{"red-the-policy-raises-the-threshold", "checks:\n  coverage: {new_code: 90}\n", 4, true, nil, exitFindings, "threshold 90%"},
		{"red-a-stale-report", "", 4, false, nil, exitFailure, "older than the Go files in scope"},
		{"near-since-the-last-tag", "", 4, true, []string{"--since-tag"}, exitClean, "base v1.0.0"},
		{"near-head-adds-nothing", "", 4, true, []string{"--base", "HEAD"}, exitClean, "new code: not applicable, no new line to cover"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newCodeFixture(t, newCodeCase{policy: tc.policy})
			report := f.write(t, "profile.cov", newCodeProfile(tc.covered), tc.fresh)
			flags := tc.flags
			if flags == nil {
				flags = []string{"--base", "base"}
			}
			code, text := f.run(t, append([]string{"--new-code", "--input", report}, flags...)...)
			if code != tc.code || !strings.Contains(text, tc.want) {
				t.Fatalf("exit = %d, output = %q", code, text)
			}
		})
	}
}

func TestCoverageNewCodeNeverAnswersCleanWithoutAReport(t *testing.T) {
	t.Parallel()
	generated := "// Code generated by protoc-gen-go. DO NOT EDIT.\n\npackage sample\n\nfunc Gen() int {\n\treturn 0\n}\n"
	for _, tc := range []struct {
		name    string
		profile string
		write   bool
		want    string
	}{
		{"red-missing", "", false, "cannot be read: write it with go test -coverprofile"},
		{"red-empty", "", true, "empty or opens without a mode line"},
		{"red-unparseable", "garbage\n", true, "empty or opens without a mode line"},
		{"red-names-no-file-of-the-universe", "mode: set\n" + newCodeModule + "/gen.pb.go:5.16,7.2 1 1\n", true, "names no file of the coverage universe"},
		{"red-a-source-the-checkout-lacks", newCodeProfile(4, newCodeModule+"/absent.go:1.1,2.2 1 1"), true, "absent.go"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newCodeFixture(t, newCodeCase{extra: map[string]string{"gen.pb.go": generated}})
			report := filepath.Join(f.repo.Root, "profile.cov")
			if tc.write {
				report = f.write(t, "profile.cov", tc.profile, true)
			}
			code, text := f.run(t, "--new-code", "--input", report, "--base", "base")
			if code != exitFailure || !strings.Contains(text, tc.want) || !strings.Contains(f.ta.stderr.String(), "unmeasured") {
				t.Fatalf("exit = %d, output = %q", code, text)
			}
		})
	}
}

func TestCoverageNewCodeBases(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		remote bool
		flags  []string
		code   int
		want   string
	}{
		{"near-the-remote-default-branch", true, nil, exitClean, "base refs/remotes/origin/main"},
		{"red-no-default-branch-and-no-base", false, nil, exitFailure, "name --base <ref> or --since-tag"},
		{"red-an-unknown-base", false, []string{"--base", "nowhere"}, exitFailure, "names no commit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newCodeFixture(t, newCodeCase{})
			if tc.remote {
				f.repo.Git("update-ref", "refs/remotes/origin/main", "base")
				f.repo.Git("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
			}
			report := f.write(t, "profile.cov", newCodeProfile(4), true)
			code, text := f.run(t, append([]string{"--new-code", "--input", report}, tc.flags...)...)
			if code != tc.code || !strings.Contains(text, tc.want) {
				t.Fatalf("exit = %d, output = %q", code, text)
			}
		})
	}
}

func TestCoverageNewCodeUsage(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"red-two-periods", []string{"--new-code", "--base", "base", "--since-tag"}, "two new code periods"},
		{"red-a-period-without-new-code", []string{"--input", "profile.cov", "--since-tag"}, "need --new-code"},
		{"red-a-second-profile", []string{"--new-code", "--plain-input", "plain.cov", "--base", "base"}, "--plain-input does not apply"},
		{"red-a-floor-flag", []string{"--new-code", "--total", "90", "--base", "base"}, "--total does not apply"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newCodeFixture(t, newCodeCase{})
			code, _ := f.run(t, tc.args...)
			if code != exitFailure || !strings.Contains(f.ta.stderr.String(), tc.want) {
				t.Fatalf("exit = %d, stderr = %q", code, f.ta.stderr.String())
			}
		})
	}
}

func TestCoverageNewCodeWithTheGoCapabilityOff(t *testing.T) {
	t.Parallel()
	f := newCodeFixture(t, newCodeCase{})
	f.repo.Commit("chore: switch the go capability off", map[string]string{
		"golaw.yml": "golaw: 1\nkit: {version: " + testVersion + "}\ncapabilities: []\n",
	})
	code, text := f.run(t, "--new-code", "--base", "base")
	if code != exitClean || !strings.Contains(text, "not applicable, the go capability is off") {
		t.Fatalf("exit = %d, output = %q", code, text)
	}
}
