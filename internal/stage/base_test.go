package stage_test

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/source"
	"github.com/cristianargotti/golaw/internal/stage"
)

// goPolicy enables the go capability, so the goast checks judge the Go
// files of a case.
const goPolicy = "golaw: 1\nkit: {version: 0.1.0}\ncapabilities: [go]\n"

// adapterDir is the root of an adapter package, whose non-test Go files
// goast.adapter-package-size holds to 12.
const adapterDir = "internal/adapters/in/http"

// goFile is a Go file of n lines: the package clause, then one declaration
// per line.
func goFile(n int) string {
	lines := make([]string, 0, n)
	lines = append(lines, "package a")
	for i := 1; i < n; i++ {
		lines = append(lines, "var v"+strconv.Itoa(i)+" = "+strconv.Itoa(i))
	}
	return strings.Join(lines, "\n") + "\n"
}

// goFunc is a Go file whose one function holds n statements, one per line.
func goFunc(n int) string {
	return "package a\n\nfunc long() {\n\tx := 0\n" + strings.Repeat("\tx++\n", n-2) + "\t_ = x\n}\n"
}

// at is one file of a case.
func at(name, content string) map[string]string { return map[string]string{name: content} }

// adapter is an adapter package of n non-test Go files.
func adapter(n int) map[string]string {
	files := make(map[string]string, n)
	for i := 1; i <= n; i++ {
		files[fmt.Sprintf("%s/f%02d.go", adapterDir, i)] = "package http\n"
	}
	return files
}

// goRepo is a repository whose base commit holds goPolicy beside the base
// files, with the staged files in its index.
func goRepo(t *testing.T, base, staged map[string]string) *gittest.Repo {
	t.Helper()
	r := gittest.New(t)
	files := map[string]string{"golaw.yml": goPolicy}
	maps.Copy(files, base)
	r.Commit("feat: base", files)
	r.Stage(staged)
	return r
}

// A goast message carries a count ("file has 321 lines", "function long has
// 44 statements", "13 non-test files in one adapter package"), so an edit
// that leaves a finding the base held changes its message; the staged
// comparison reads every count as one mark, and any other digit, as one of
// a name, as itself. A directory a commit only adds a file to is asked at
// the base too, so its count is compared like any other.
func TestScanKeepsAFindingWhoseCountOnlyChanged(t *testing.T) {
	t.Parallel()
	held := "package a\n\ntype Auth1 struct{ Token string }\n\ntype Auth2 struct{ TokenID string }\n"
	moved := "package a\n\ntype Auth1 struct{ TokenID string }\n\ntype Auth2 struct{ Token string }\n"
	cases := map[string]struct {
		base, staged map[string]string
		want         []string
	}{
		"a 320-line file grown to 321":                {at("a.go", goFile(320)), at("a.go", goFile(321)), nil},
		"a 320-line file shortened to 310":            {at("a.go", goFile(320)), at("a.go", goFile(310)), nil},
		"a 45-line function shortened to 44":          {at("a.go", goFunc(45)), at("a.go", goFunc(44)), nil},
		"a 300-line file grown to 301":                {at("a.go", goFile(300)), at("a.go", goFile(301)), []string{"a.go"}},
		"a credential moved to a name a digit off":    {at("a.go", held), at("a.go", moved), []string{"a.go"}},
		"an adapter package of 13 files given a 14th": {adapter(13), at(adapterDir+"/f14.go", "package http\n"), nil},
		"an adapter package of 12 files given a 13th": {adapter(12), at(adapterDir+"/f13.go", "package http\n"), []string{adapterDir}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := goRepo(t, tc.base, tc.staged)
			if got := findingPaths(scan(t, r, source.IndexOptions{}).Decision); !slices.Equal(got, tc.want) {
				t.Errorf("findings at %v, want %v", got, tc.want)
			}
		})
	}
}

// The files beside a new file are judged at the base in a run of their
// own, and a staged file at its own path as the index judges it: a file
// there that no check can judge never holds a staged finding back, and a
// package finding a check reports at the first file it meets stays where
// both runs meet it.
func TestScanJudgesTheFilesBesideANewFileApart(t *testing.T) {
	t.Parallel()
	start := func(name string) string { return "package p\n\nfunc " + name + "() { go " + name + "() }\n" }
	cases := map[string]struct {
		base, staged map[string]string
		want         []string
	}{
		"a new file beside one that does not parse": {
			at("p/broken.go", "package p\n\nfunc {\n"), at("p/new.go", strings.Replace(goFunc(45), "package a", "package p", 1)), []string{"p/new.go"},
		},
		"the second of two goroutine starts edited": {
			map[string]string{"p/a.go": start("a"), "p/b.go": start("b")}, at("p/b.go", start("b")+"\nvar x = 1\n"), nil,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := goRepo(t, tc.base, tc.staged)
			if got := findingPaths(scan(t, r, source.IndexOptions{}).Decision); !slices.Equal(got, tc.want) {
				t.Errorf("findings at %v, want %v", got, tc.want)
			}
		})
	}
}

// A base file over the read bound holds findings no check could read, so a
// finding at its path cannot be proven new: it is held back, never reported
// as the commit's own, and the check stays unmeasured, never clean.
func TestScanHoldsBackWhatAnUnreadBaseCannotDisprove(t *testing.T) {
	t.Parallel()
	big := "one " + dash + " two\n" + strings.Repeat("a\n", model.MaxReadBytes/2)
	short := "one " + dash + " two\n"
	cases := map[string]struct {
		base, staged map[string]string
		want         []string
		unmeasured   bool
	}{
		"an unread base shortened, its dash kept": {map[string]string{"big.md": big}, map[string]string{"big.md": short}, nil, true},
		"beside it, a new file with a dash": {
			map[string]string{"big.md": big}, map[string]string{"big.md": short, "new.md": "a " + dash + " b\n"}, []string{"new.md"}, true,
		},
		"a readable base shortened, its dash kept": {map[string]string{"big.md": short + "three\n"}, map[string]string{"big.md": short}, nil, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := gittest.New(t)
			r.Commit("docs: base", tc.base)
			r.Stage(tc.staged)
			res := scan(t, r, source.IndexOptions{})
			if got := findingPaths(res.Decision); !slices.Equal(got, tc.want) {
				t.Errorf("findings at %v, want %v", got, tc.want)
			}
			held := slices.ContainsFunc(res.Decision.Unmeasured, func(u engine.Unmeasured) bool {
				return u.Check == "text.dashes" && strings.Contains(u.Reason, "big.md")
			})
			if held != tc.unmeasured {
				t.Errorf("text.dashes held back = %v, want %v: %+v", held, tc.unmeasured, res.Decision.Unmeasured)
			}
		})
	}
}

// An exception makes a finding a note and names its record in the message
// after the engine fingerprinted it, so a kept finding keeps the
// fingerprint the engine gave it, the one golaw scan reports.
func TestScanKeepsTheFingerprintTheEngineGaveAFinding(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	excepted := "golaw: 1\nkit: {version: 0.1.0}\nexceptions: [{check: text.dashes, path: b.md, record: r.md, until: 2026-12-31}]\n"
	r.Commit("docs: base", map[string]string{"golaw.yml": excepted, "a.md": "a " + dash + " a\n"})
	r.Stage(map[string]string{"a.md": "a " + dash + " a\nmore\n", "b.md": "b " + dash + " b\n", "c.md": "c " + dash + " c\n"})
	in := input(t, r, source.IndexOptions{})
	res, err := stage.Scan(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	direct, err := engine.Dispatch(t.Context(), in.Registry, res.Input)
	if err != nil {
		t.Fatal(err)
	}
	all := engine.Decide(direct).Findings
	if len(all) != 3 || all[1].Severity != model.SeverityNote || !slices.Equal(res.Decision.Findings, all[1:]) {
		t.Errorf("findings %+v, want the engine's %+v after the one a.md carries along", res.Decision.Findings, all)
	}
}
