package stage_test

import (
	"context"
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

// at keeps a row of one file on one line.
func at(name, content string) map[string]string { return map[string]string{name: content} }

// adapter builds an adapter package of n non-test Go files, so a case
// stands on either side of the limit of goast.adapter-package-size.
func adapter(n int) map[string]string {
	files := make(map[string]string, n)
	for i := 1; i <= n; i++ {
		files[fmt.Sprintf("%s/f%02d.go", adapterDir, i)] = "package http\n"
	}
	return files
}

// goRepo commits goPolicy beside the base files, so the goast checks judge
// both sides, and stages the files of the case.
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

// The files beside a new file, and the other Go files of a staged
// package, are judged at the base in a run of their own, and a staged file
// at its own path as the index judges it: a file there that no check can
// judge never holds a staged finding back, and a package finding a check
// reports at the first file it meets is the one the base holds wherever
// in the package each run meets it.
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
		"a start added beside the one the base holds": {
			map[string]string{"p/a.go": start("a"), "p/b.go": "package p\n"}, at("p/b.go", start("b")), nil,
		},
		"the first start of a package": {
			map[string]string{"p/a.go": "package p\n", "p/b.go": "package p\n"}, at("p/b.go", start("b")), []string{"p/b.go"},
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
// finding at its path, or at a file beside it that a check reads with it,
// cannot be proven new: it is held back, never reported as the commit's
// own, and the check stays unmeasured, never clean, with the files behind
// the doubt among its unread files as the index names them.
func TestScanHoldsBackWhatAnUnreadBaseCannotDisprove(t *testing.T) {
	t.Parallel()
	big := "one " + dash + " two\n" + strings.Repeat("a\n", model.MaxReadBytes/2)
	short := "one " + dash + " two\n"
	cfg := "package p\n\ntype Cfg struct{ Password string }\n"
	field := map[string]string{"golaw.yml": goPolicy, "p/a.go": cfg, "p/z.go": "package p\n" + strings.Repeat("a\n", model.MaxReadBytes/2)}
	shortened := map[string]string{"p/z.go": "package p\n", "p/a.go": cfg + "\nvar x = 1\n"}
	beside := maps.Clone(shortened)
	beside["p/b.go"] = "package p\n\ntype Other struct{ Secret string }\n"
	cases := map[string]struct {
		base, staged map[string]string
		moved        [2]string
		check        string
		want, unread []string
		reason       string
	}{
		"an unread base shortened, its dash kept": {
			base: at("big.md", big), staged: at("big.md", short), check: "text.dashes", unread: []string{"big.md"}, reason: "compare with the base: read big.md",
		},
		"beside it, a new file with a dash": {
			base: at("big.md", big), staged: map[string]string{"big.md": short, "new.md": "a " + dash + " b\n"}, check: "text.dashes",
			want: []string{"new.md"}, unread: []string{"big.md"}, reason: "compare with the base: read big.md",
		},
		"a readable base shortened, its dash kept": {base: at("big.md", short+"three\n"), staged: at("big.md", short), check: "text.dashes"},
		"beside it, a staged file over the bound": {
			base: at("big.md", big), staged: map[string]string{"big.md": short, "huge.md": big}, check: "text.dashes",
			unread: []string{"big.md", "huge.md"}, reason: "read huge.md",
		},
		"an unread base moved and shortened, its dash kept": {
			base: at("big.md", big), moved: [2]string{"big.md", "docs/big.md"}, staged: at("docs/big.md", short+strings.Repeat("a\n", model.MaxReadBytes/2-64)),
			check: "text.dashes", unread: []string{"docs/big.md"}, reason: "compare with the base: read big.md",
		},
		"a credential beside an unread base file": {
			base: field, staged: shortened, check: "goast.secret-fields", unread: []string{"p/z.go"}, reason: "compare with the base: read p/z.go",
		},
		"beside them, a new file with a credential": {
			base: field, staged: beside, check: "goast.secret-fields", want: []string{"p/b.go"}, unread: []string{"p/z.go"}, reason: "compare with the base: read p/z.go",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := gittest.New(t)
			r.Commit("docs: base", tc.base)
			if tc.moved[0] != "" {
				r.StageRename(tc.moved[0], tc.moved[1])
			}
			r.Stage(tc.staged)
			res := scan(t, r, source.IndexOptions{})
			if got := findingPaths(res.Decision); !slices.Equal(got, tc.want) {
				t.Errorf("findings at %v, want %v", got, tc.want)
			}
			got := outcomeOf(t, res, tc.check)
			unread, measured := slices.Sorted(slices.Values(got.Unread)), got.Status.Kind == model.StatusMeasured
			if !slices.Equal(unread, tc.unread) || measured != (tc.reason == "") || !strings.Contains(got.Status.Reason, tc.reason) {
				t.Errorf("%s: status %+v, unread %v; want unread %v and a reason with %q", tc.check, got.Status, unread, tc.unread, tc.reason)
			}
		})
	}
}

// outcomeOf finds one check in the run, because a finding held back shows
// in its status and its unread files, never among the findings.
func outcomeOf(t *testing.T, res stage.Result, check string) model.Result {
	t.Helper()
	for i := range res.Run.Outcomes {
		if res.Run.Outcomes[i].Spec.ID == check {
			return res.Run.Outcomes[i].Result
		}
	}
	t.Fatalf("no outcome of %s", check)
	return model.Result{}
}

// stopped is a check that stops over the tree of a commit and finds every
// target of the index, so the base of a staged scan stops for another
// reason than the read bound.
type stopped struct{}

func (stopped) Spec() model.Spec {
	return model.Spec{ID: "probe.stopped", Rule: "rules/1@1", Scope: model.ScopeFile, Owner: model.OwnerCore, Proof: "probe.stopped"}
}

func (stopped) Run(_ context.Context, in model.Input) (model.Result, error) {
	if _, base := in.Snapshot.(*source.Revision); base {
		return model.Result{Status: model.Unmeasured("the base stopped")}, nil
	}
	findings := make([]model.Finding, 0, len(in.Targets))
	for _, f := range in.Targets {
		findings = append(findings, model.Finding{Path: f.Path, Code: "probe", Message: "found", Severity: model.SeverityDeny})
	}
	return model.Result{Status: model.Measured(), Findings: findings}, nil
}

// A base that stopped before it measured disproves no finding, so every
// finding of the check is held back and the check stays unmeasured with
// the reason of the base.
func TestScanHoldsBackWhatABaseThatStoppedCannotDisprove(t *testing.T) {
	t.Parallel()
	builder := engine.New()
	if err := builder.Register(stopped{}); err != nil {
		t.Fatal(err)
	}
	registry, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	r := gittest.New(t)
	r.Commit("docs: base", map[string]string{"a.md": "a\n"})
	r.Stage(map[string]string{"a.md": "b\n", "new.md": "c\n"})
	in := input(t, r, source.IndexOptions{})
	in.Registry = registry
	res, err := stage.Scan(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	unmeasured := res.Decision.Unmeasured
	if got := findingPaths(res.Decision); len(got) != 0 || len(unmeasured) != 1 || unmeasured[0].Reason != "compare with the base: the base stopped" {
		t.Errorf("findings at %v, unmeasured %+v", got, unmeasured)
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
