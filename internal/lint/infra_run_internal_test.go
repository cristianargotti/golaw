package lint

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/hometest"
	"github.com/cristianargotti/golaw/internal/proof"
	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

const (
	// infraTimeout bounds one tool run of a test; the hook budget of 1.5 s
	// is proven by the acceptance lane on warm runners, not here.
	infraTimeout = 2 * time.Minute
	// infraDeadProxy refuses every connection, so a tool that tried the
	// network fails its test instead of reaching it.
	infraDeadProxy = "http://127.0.0.1:9"
	infraDirMode   = 0o750
	infraFileMode  = 0o600
)

// infraByTool answers the driver of a tool among drivers.
func infraByTool(t *testing.T, drivers []Driver, tool string) Driver {
	t.Helper()
	for _, d := range drivers {
		if d.Tool == tool {
			return d
		}
	}
	t.Fatalf("no driver for %s", tool)
	return Driver{}
}

// infraEntryDriver answers the driver a proof entry lint.<tool>-<code> names.
func infraEntryDriver(t *testing.T, drivers []Driver, e proof.Entry) Driver {
	t.Helper()
	for _, d := range drivers {
		if e.ID == "lint."+d.Tool+"-"+e.Code {
			return d
		}
	}
	t.Fatalf("%s names no driver of this test", e.ID)
	return Driver{}
}

// infraUsable resolves the pinned tool, or skips with the reason the
// acceptance lane records.
func infraUsable(t *testing.T, name string) toolchain.Resolved {
	t.Helper()
	tool, ok := toolchain.Lookup(name)
	if !ok {
		t.Fatalf("%s is not pinned in toolchain.yml", name)
	}
	resolved := testResolver(t).Full(t.Context(), tool)
	if !resolved.Usable() {
		t.Skipf("%s %s not installed: run golaw setup --surface", name, tool.Version)
	}
	return resolved
}

// infraMaterialize writes the set of a driver into a fresh tool directory.
func infraMaterialize(t *testing.T, d Driver, set string) string {
	t.Helper()
	dir, err := toolchain.Materialize(t.TempDir(), infraKit, set, d.Config)
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// infraWrite writes a tree under a new root and answers the root and the
// sorted slash paths of the files whose language the driver reads.
func infraWrite(t *testing.T, d Driver, files map[string][]byte) (string, []string) {
	t.Helper()
	root := t.TempDir()
	paths := make([]string, 0, len(files))
	for rel, data := range files {
		target := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), infraDirMode); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, data, infraFileMode); err != nil {
			t.Fatal(err)
		}
		info, err := surface.Detect(rel, data)
		if err != nil {
			t.Fatalf("detect %s: %v", rel, err)
		}
		if slices.Contains(d.Languages, info.Language) {
			paths = append(paths, rel)
		}
	}
	slices.Sort(paths)
	return root, paths
}

// infraOffline is the environment of a tool under test: an isolated home,
// the tool's own directory as PATH and proxies that refuse every connection.
func infraOffline(t *testing.T, resolved toolchain.Resolved) []string {
	t.Helper()
	home := hometest.New(t)
	env := toolchain.ToolEnv(append(home.Environ(), "PATH="+filepath.Dir(resolved.Path)))
	return append(env, "HTTPS_PROXY="+infraDeadProxy, "HTTP_PROXY="+infraDeadProxy, "ALL_PROXY="+infraDeadProxy)
}

// infraExec runs one request and parses its output against root.
func infraExec(t *testing.T, d Driver, req toolchain.Request, root string) []Issue {
	t.Helper()
	execution, err := toolchain.Exec(t.Context(), req)
	if err != nil {
		t.Fatalf("%s: %v", d.Tool, err)
	}
	if execution.Killed {
		t.Fatalf("%s ran past %s", d.Tool, req.Timeout)
	}
	issues, err := Parse(d.Format, d.Tool, execution.Output, root)
	if err != nil {
		t.Fatalf("%s output: %v: %s", d.Tool, err, execution.Output.Stderr)
	}
	return issues
}

// infraRun runs a driver under a set over a tree, starting the tool in the
// tree's root so a repository configuration there is in its way, and
// answers the issues; false when the set gives the tool nothing to run,
// which needs no tool at all.
func infraRun(t *testing.T, d Driver, set string, files map[string][]byte) ([]Issue, bool) {
	t.Helper()
	root, paths := infraWrite(t, d, files)
	args := d.Args(Input{Root: root, Files: paths, ConfigDir: infraMaterialize(t, d, set), Set: set})
	if args == nil {
		return nil, false
	}
	resolved := infraUsable(t, d.Tool)
	req := toolchain.Request{Path: resolved.Path, Args: args, Dir: root, Env: infraOffline(t, resolved), Timeout: infraTimeout}
	return infraExec(t, d, req, root), true
}

// infraHook runs a driver's hook arguments with the bytes on stdin from the
// tool directory, as the post event does.
func infraHook(t *testing.T, d Driver, set, path, shell string, content []byte) []Issue {
	t.Helper()
	if d.HookArgs == nil {
		t.Fatalf("%s has no hook arguments", d.Tool)
	}
	resolved := infraUsable(t, d.Tool)
	dir := infraMaterialize(t, d, set)
	args := d.HookArgs(HookInput{Path: path, Shell: shell, ConfigDir: dir})
	req := toolchain.Request{Path: resolved.Path, Args: args, Stdin: content, Dir: dir, Env: infraOffline(t, resolved), Timeout: infraTimeout}
	return infraExec(t, d, req, dir)
}

// infraCodes counts the issues by finding code.
func infraCodes(issues []Issue) map[string]int {
	counts := map[string]int{}
	for _, issue := range issues {
		counts[Code(issue.Rule)]++
	}
	return counts
}

// proveInfraSet runs every entry of a manifest through its driver under one
// set: the red tree yields the entry's code, the near tree does not.
func proveInfraSet(t *testing.T, manifest, set string, drivers []Driver) {
	t.Helper()
	m, err := proof.LoadManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range m.Checks {
		t.Run(set+"/"+e.ID, func(t *testing.T) {
			t.Parallel()
			d := infraEntryDriver(t, drivers, e)
			infraSide(t, d, set, filepath.Join(filepath.Dir(manifest), e.Red), e.Code, true)
			infraSide(t, d, set, filepath.Join(filepath.Dir(manifest), e.Near), e.Code, false)
		})
	}
}

// infraSide proves one tree of an entry.
func infraSide(t *testing.T, d Driver, set, dir, code string, red bool) {
	t.Helper()
	fixture, err := proof.LoadFixture(dir)
	if err != nil {
		t.Fatal(err)
	}
	issues, ran := infraRun(t, d, set, fixture.Files)
	if !ran {
		t.Fatalf("%s runs nothing under %s for %s", d.Tool, set, dir)
	}
	if got := infraCodes(issues)[code]; (got > 0) != red {
		t.Errorf("%s under %s: %s yields %s %d times, want red=%t: %+v", d.Tool, set, dir, code, got, red, issues)
	}
}
