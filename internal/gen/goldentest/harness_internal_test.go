package goldentest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/genheader"
	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/source"
)

// regenerated is a regenerable file of the outside harness: a profile of
// a module that does not build holds none of its lines.
const regenerated = "internal/x/zz_generated_x.go"

func regeneratedSource() []byte {
	header := genheader.Header{Version: "0.3.0", Kind: "adapter-http", Template: 1}.Line()
	return []byte(header + "\n\npackage x\n\n// Answer answers.\nfunc Answer() int {\n\treturn 1\n}\n")
}

// outside is a harness whose module root is a directory git does not
// know, so every step that reads the module meets its failure path.
func outside(t *testing.T) *harness {
	t.Helper()
	return &harness{
		tb: t, s: Slice{Name: "outside", Module: "example.com/slice"},
		repo: &gittest.Repo{Root: t.TempDir()}, policy: &model.EffectivePolicy{}, work: t.TempDir(),
		profile: filepath.Join(t.TempDir(), "absent.cov"),
		report:  Report{Files: map[string][]byte{".golaw/gen.yml": []byte("schema: 1\n"), regenerated: regeneratedSource()}, Skipped: map[Step]string{}},
	}
}

func TestEveryStepFailsOverADirectoryOutsideGit(t *testing.T) {
	t.Parallel()
	steps := map[string]func(h *harness){
		"scan":       func(h *harness) { h.scan(t.Context()) },
		"surface":    func(h *harness) { h.surface(t.Context()) },
		"dup":        func(h *harness) { h.duplication(t.Context()) },
		"coverage":   func(h *harness) { h.coverage(t.Context()) },
		"open":       func(h *harness) { h.openCoverage(t.Context()) },
		"check":      func(h *harness) { h.checkClean(t.Context()) },
		"check-open": func(h *harness) { h.checkOpen(t.Context()) },
		"fill":       func(h *harness) { h.fill(t.Context()) },
		"command":    func(h *harness) { h.generate(t.Context()) },
		"lint-config": func(h *harness) {
			if _, err := h.lintConfig(false); err != nil {
				h.failf("lint", "%v", err)
			}
		},
		"go": func(h *harness) { h.goStep(t.Context(), StepBuild, "nosuchcommand") },
	}
	for name, step := range steps {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := outside(t)
			h.s.Commands = []Command{{Kind: "adapter-http"}}
			step(h)
			if len(h.failures) == 0 {
				t.Fatalf("%s over a directory outside git failed nothing", name)
			}
		})
	}
}

func TestTheMeasuresJudgeWhatTheProfileLeavesOut(t *testing.T) {
	t.Parallel()
	declarations := []byte("package p\n\n// Answer is a constant.\nconst Answer = 1\n")
	statements := []byte("package p\n\n// Answer answers.\nfunc Answer() int {\n\treturn 1\n}\n")
	if hasStatements("p.go", declarations) || !hasStatements("p.go", statements) || !hasStatements("p.go", []byte("package")) {
		t.Error("hasStatements judges a file of declarations, of statements or one that does not parse wrongly")
	}
	h := outside(t)
	h.report.Files = map[string][]byte{"p/decl.go": declarations, "p/stmt.go": statements}
	h.judgeOpen("p/decl.go", nil)
	h.judgeOpen("p/stmt.go", nil)
	if len(h.failures) != 1 || !strings.Contains(h.failures[0], "p/stmt.go: no test of its package covers this regenerable file") {
		t.Errorf("failures = %v", h.failures)
	}
}

func TestTheSmallHelpersAnswerTheirEdges(t *testing.T) {
	t.Parallel()
	if _, err := replaceBody("p.go", []byte("package p\n\nvar x = 1\n"), 3, "return"); !errors.Is(err, ErrFills) {
		t.Errorf("replaceBody outside a function = %v", err)
	}
	if _, err := replaceBody("p.go", []byte("package"), 1, "return"); !errors.Is(err, ErrFills) {
		t.Errorf("replaceBody of a file that does not parse = %v", err)
	}
	if path, reason := pinned(t.Context(), "nosuch"); path != "" || reason != "nosuch is not a pinned tool" {
		t.Errorf("pinned(nosuch) = %q, %q", path, reason)
	}
	if got := goLine([]byte("module example.com/slice\n")); got != "" {
		t.Errorf("goLine without a go directive = %q", got)
	}
	parent := map[string]any{"run": "flat"}
	section(parent, "run")["mode"] = "x"
	if _, ok := parent["run"].(map[string]any); !ok {
		t.Error("section kept a value that is no mapping")
	}
	if _, err := compile([]byte("golaw: [")); err == nil {
		t.Error("compile accepted a golaw.yml that does not decode")
	}
	if _, err := compile([]byte("golaw: 1\nkit: { version: 0.3.0 }\ncapabilities: [nosuch]\n")); err == nil {
		t.Error("compile accepted an unknown capability")
	}
}

func TestValidateNamesWhatTheSliceLacks(t *testing.T) {
	t.Parallel()
	noGoMod := gentest.Contracts()
	delete(noGoMod, goModFile)
	tests := map[string]struct {
		s    Slice
		want string
	}{
		"red: no go.mod": {Slice{Module: "example.com/slice", Base: noGoMod}, "the base holds no go.mod"},
		"red: no kind":   {Slice{Module: "example.com/slice", Base: gentest.Contracts()}, "no kind"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := validate(tc.s); !errors.Is(err, ErrSlice) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("validate = %v, want ErrSlice holding %q", err, tc.want)
			}
		})
	}
}

func TestLoadFillsRefusesAFillWithoutItsID(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "fills.yml")
	if err := os.WriteFile(path, []byte("- { body: return }\n"), fillMode); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFills(path); !errors.Is(err, ErrFills) || !strings.Contains(err.Error(), "a fill has no id") {
		t.Fatalf("LoadFills = %v", err)
	}
}

// inGit is a harness over a real module of the growth fixture.
func inGit(t *testing.T) *harness {
	t.Helper()
	h := outside(t)
	h.repo = gentest.Module(t, gentest.Contracts())
	policy := model.CoreDefaults()
	h.policy = &policy
	return h
}

// racing writes the file it renders on disk itself, the shape of an edit
// that lands between the plan and its writes.
type racing struct{ root string }

func (racing) Name() string  { return genkind.KindAdapterHTTP }
func (racing) Template() int { return 1 }

func (r racing) New(_ context.Context, _ genkind.Input, _ genkind.Request) (genkind.Output, error) {
	const path = "internal/racing/zz_generated_racing.go"
	if err := os.MkdirAll(filepath.Dir(source.OSPath(r.root, path)), 0o700); err != nil {
		return genkind.Output{}, fmt.Errorf("racing directory: %w", err)
	}
	if err := os.WriteFile(source.OSPath(r.root, path), []byte("package racing\n"), fillMode); err != nil {
		return genkind.Output{}, fmt.Errorf("racing file: %w", err)
	}
	content := regeneratedSource()
	return genkind.Output{
		Entry:       manifest.Entry{Kind: genkind.KindAdapterHTTP, Template: "adapter-http@1", Name: "racing.Racing", Regenerable: []string{path}},
		Regenerable: []genkind.File{{Path: path, Content: content}},
	}, nil
}

func (r racing) Regenerate(ctx context.Context, in genkind.Input, _ manifest.Entry) (genkind.Output, error) {
	return r.New(ctx, in, genkind.Request{})
}

func TestTheStepsNameWhatTheyCouldNotDo(t *testing.T) {
	t.Parallel()
	t.Run("a slice without a surface file", func(t *testing.T) {
		t.Parallel()
		h := inGit(t)
		h.report.Files = map[string][]byte{regenerated: regeneratedSource()}
		h.surface(t.Context())
		if h.report.Skipped[StepSurface] != noSurface || len(h.failures) != 0 {
			t.Errorf("skipped %v, failures %v", h.report.Skipped, h.failures)
		}
	})
	t.Run("a surface set the linters do not know", func(t *testing.T) {
		t.Parallel()
		h := inGit(t)
		h.surfaceSet = "nosuch"
		h.surface(t.Context())
		if len(h.failures) == 0 || !strings.HasPrefix(h.failures[0], "surface: ") {
			t.Errorf("failures = %v", h.failures)
		}
	})
	t.Run("a scan without a policy", func(t *testing.T) {
		t.Parallel()
		h := inGit(t)
		h.policy = nil
		h.scan(t.Context())
		if len(h.failures) == 0 || !strings.HasPrefix(h.failures[0], "scan: ") {
			t.Errorf("failures = %v", h.failures)
		}
	})
	t.Run("a write the tree changed under", func(t *testing.T) {
		t.Parallel()
		h := inGit(t)
		h.s.Kinds = []genkind.Kind{racing{root: h.repo.Root}}
		h.policy.Generate = model.DefaultGenerate()
		h.manifest = manifest.Manifest{Schema: manifest.Schema, Support: manifest.Support{Path: "internal/gensupport", Template: "support@1"}}
		if _, ok := h.command(t.Context(), "command racing", &Command{Kind: genkind.KindAdapterHTTP}); ok || len(h.failures) == 0 ||
			!strings.Contains(h.failures[0], "command racing: writes skipped: internal/racing/zz_generated_racing.go") {
			t.Errorf("failures = %v", h.failures)
		}
	})
}

func TestTheLintStepsNameAFileTheyCouldNotJudge(t *testing.T) {
	t.Parallel()
	for name, step := range map[Step]func(h *harness){
		StepLint:   func(h *harness) { h.lint(t.Context()) },
		StepFormat: func(h *harness) { h.format(t.Context()) },
	} {
		t.Run(string(name), func(t *testing.T) {
			t.Parallel()
			h := inGit(t)
			if err := os.WriteFile(source.OSPath(h.repo.Root, lintAsset), []byte("linters: {}\n"), fillMode); err != nil {
				t.Fatal(err)
			}
			h.report.Files = map[string][]byte{regenerated: regeneratedSource()}
			step(h)
			if reason, skipped := h.report.Skipped[name]; skipped {
				t.Skip(reason)
			}
			if len(h.failures) == 0 || !strings.HasPrefix(h.failures[0], string(name)+": ") {
				t.Errorf("failures = %v", h.failures)
			}
		})
	}
	h := outside(t)
	h.lint(t.Context())
	if _, skipped := h.report.Skipped[StepLint]; !skipped && len(h.failures) == 0 {
		t.Error("lint over a module without its lint asset neither skipped nor failed")
	}
}

func TestLintConfigNamesAnAssetItCannotDecode(t *testing.T) {
	t.Parallel()
	h := inGit(t)
	if err := os.WriteFile(source.OSPath(h.repo.Root, lintAsset), []byte("linters: [\n"), fillMode); err != nil {
		t.Fatal(err)
	}
	if _, err := h.lintConfig(false); err == nil || !strings.Contains(err.Error(), "decode "+lintAsset) {
		t.Fatalf("lintConfig = %v, want the decode failure of %s", err, lintAsset)
	}
}
