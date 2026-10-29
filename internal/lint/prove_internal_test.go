package lint

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/proof"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// testResolver answers the resolver the asset tests use: the directory
// GOLAW_TOOLS_DIR names when set, else policy.ToolsDir of the process
// environment, else none, so a machine without the tools skips and never
// fails. The tests only execute what they find there; they write nothing in
// it.
func testResolver(t *testing.T) toolchain.Resolver {
	t.Helper()
	dir := os.Getenv("GOLAW_TOOLS_DIR")
	resolved, err := policy.ToolsDir(runtime.GOOS, os.Getenv)
	if dir == "" && err == nil {
		dir = resolved
	}
	return toolchain.Resolver{Dir: dir, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Env: os.Environ(), Run: runner.Run}
}

// proveManifest runs every entry of a manifest through the driver of its
// tool: the red tree yields the code, the near tree does not, under the
// strict set and, when the rule map row of the code is a parity row, the red
// tree yields it under parity too. A tool that resolves to no usable path
// skips the entry with the reason the acceptance lane records; the decoding
// of the manifest, the rule map and the trees never skips.
func proveManifest(t *testing.T, manifest string, drivers []Driver) {
	t.Helper()
	m, err := proof.LoadManifest(manifest)
	if err != nil {
		t.Fatalf("load %s: %v", manifest, err)
	}
	rows := manifestRows(t, manifest)
	resolver := testResolver(t)
	for _, entry := range m.Checks {
		t.Run(entry.ID, func(t *testing.T) {
			t.Parallel()
			d, ok := driverOf(entry.ID, drivers)
			if !ok {
				t.Fatalf("no driver serves %s", entry.ID)
			}
			if reason := proveEntry(t, filepath.Dir(manifest), entry, d, rows, resolver); reason != "" {
				t.Skip(reason)
			}
		})
	}
}

// manifestRows reads rulemap/<name> beside the proof directory of
// proof/<name>, none when the part has no rule map yet.
func manifestRows(t *testing.T, manifest string) []Row {
	t.Helper()
	path := filepath.Join(filepath.Dir(filepath.Dir(manifest)), "rulemap", filepath.Base(manifest))
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	rows, err := DecodeRows(data)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return rows
}

// driverOf picks the driver whose tool prefixes the entry id, the longest
// tool name first, so cfn-lint wins over a tool named cfn.
func driverOf(id string, drivers []Driver) (Driver, bool) {
	rest := strings.TrimPrefix(id, "lint.")
	best, found := Driver{}, false
	for _, d := range drivers {
		if strings.HasPrefix(rest, d.Tool+"-") && len(d.Tool) > len(best.Tool) {
			best, found = d, true
		}
	}
	return best, found
}

// proveEntry proves one entry and answers the skip reason, empty when it ran.
func proveEntry(t *testing.T, dir string, entry proof.Entry, d Driver, rows []Row, resolver toolchain.Resolver) string {
	t.Helper()
	red, err := proof.LoadFixture(filepath.Join(dir, entry.Red))
	if err != nil {
		t.Fatal(err)
	}
	near, err := proof.LoadFixture(filepath.Join(dir, entry.Near))
	if err != nil {
		t.Fatal(err)
	}
	tool, ok := toolchain.Lookup(d.Tool)
	if !ok {
		t.Fatalf("%s is no pinned tool", d.Tool)
	}
	resolved := resolver.Full(t.Context(), tool)
	if !resolved.Usable() {
		return fmt.Sprintf("%s %s not installed: run golaw setup --surface", tool.Name, tool.Version)
	}
	if !slices.Contains(codesOf(t, red, d, resolved.Path, model.SurfaceStrict), entry.Code) {
		t.Errorf("red %s does not yield %s under strict", entry.Red, entry.Code)
	}
	if slices.Contains(codesOf(t, near, d, resolved.Path, model.SurfaceStrict), entry.Code) {
		t.Errorf("near miss %s yields %s", entry.Near, entry.Code)
	}
	if parityRow(rows, d.Tool, entry.Code) && !slices.Contains(codesOf(t, red, d, resolved.Path, model.SurfaceParity), entry.Code) {
		t.Errorf("red %s does not yield %s under parity, where its row puts it", entry.Red, entry.Code)
	}
	return ""
}

func parityRow(rows []Row, tool, code string) bool {
	return slices.ContainsFunc(rows, func(r Row) bool {
		return r.Tool == tool && Code(r.Rule) == code && r.Set == model.SurfaceParity
	})
}

// codesOf writes a fixture tree, runs the driver's lint invocation over the
// files of its languages and answers the codes of the issues.
func codesOf(t *testing.T, fixture proof.Fixture, d Driver, path, set string) []string {
	t.Helper()
	root := t.TempDir()
	files := writeFixture(t, root, fixture, d)
	config := ""
	if d.Config != nil {
		dir, err := toolchain.Materialize(t.TempDir(), "test", set, d.Config)
		if err != nil {
			t.Fatal(err)
		}
		config = dir
	}
	run, err := toolchain.Exec(t.Context(), toolchain.Request{
		Path: path, Args: d.Args(Input{Root: root, Files: files, ConfigDir: config, Set: set}),
		Dir: root, Env: toolchain.ToolEnv(os.Environ()), Timeout: ToolTimeout,
	})
	if err != nil || run.Killed {
		t.Fatalf("run %s: %+v, %v", d.Tool, run, err)
	}
	issues, err := Parse(d.Format, d.Tool, run.Output, root)
	if err != nil && !errors.Is(err, ErrUnparsed) {
		t.Fatalf("parse %s: %v", d.Tool, err)
	}
	codes := make([]string, 0, len(issues))
	for _, issue := range issues {
		codes = append(codes, Code(issue.Rule))
	}
	return codes
}

// writeFixture writes a fixture tree under root and answers, sorted, the
// files of the driver's languages.
func writeFixture(t *testing.T, root string, fixture proof.Fixture, d Driver) []string {
	t.Helper()
	var files []string
	for name, content := range fixture.Files {
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, content, 0o600); err != nil {
			t.Fatal(err)
		}
		info, err := surface.Detect(name, content)
		if err == nil && slices.Contains(d.Languages, info.Language) {
			files = append(files, name)
		}
	}
	slices.Sort(files)
	return files
}

// ruffEval is the driver the self-test proves the harness with: the
// isolated ruff check of one rule, no kit configuration needed.
func ruffEval() Driver {
	return Driver{
		Tool: "ruff", Languages: []surface.Language{surface.Python}, Format: FormatRuff,
		Args: func(in Input) []string {
			return append([]string{"check", "--isolated", "--no-cache", "--select", "S307", "--output-format", "json"}, in.Files...)
		},
	}
}

func TestProveManifestRunsTheRealToolWhenItIsInstalled(t *testing.T) {
	t.Parallel()
	proveManifest(t, filepath.Join("testdata", "prove", "proof", "self.yml"), []Driver{ruffEval()})
}

func TestProveEntrySkipsAToolMissingOnThisMachine(t *testing.T) {
	t.Parallel()
	m, err := proof.LoadManifest(filepath.Join("testdata", "prove", "proof", "self.yml"))
	if err != nil {
		t.Fatal(err)
	}
	none := toolchain.Resolver{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Env: []string{"PATH=" + t.TempDir()}, Run: runner.Run}
	reason := proveEntry(t, filepath.Join("testdata", "prove", "proof"), m.Checks[0], ruffEval(), nil, none)
	if reason != "ruff 0.17.1 not installed: run golaw setup --surface" {
		t.Errorf("reason = %q", reason)
	}
}

func TestDriverOfPrefersTheLongestToolName(t *testing.T) {
	t.Parallel()
	drivers := []Driver{{Tool: "cfn"}, {Tool: "cfn-lint"}, {Tool: "ruff"}}
	if d, ok := driverOf("lint.cfn-lint-e3002", drivers); !ok || d.Tool != "cfn-lint" {
		t.Errorf("driverOf = %+v, %t", d, ok)
	}
	if _, ok := driverOf("lint.eslint-no-undef", drivers); ok {
		t.Error("an entry of a tool without a driver found one")
	}
}
