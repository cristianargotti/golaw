// Package toolchaintest installs fake pinned tools into a temporary tool
// directory at the path the layout names, with the receipt a real install
// writes, so the resolver, the lint dispatch, doctor and the hook see an
// installed tool without the network.
package toolchaintest

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// maxShebang bounds what Calls reads of a fake to find its #! line.
const maxShebang = 512

// dirMode keeps the fake's directory the developer's own, as setup does.
const dirMode = 0o700

// scriptMode lets the kernel start a fake #! script, as pip leaves one.
const scriptMode = 0o700

// venvPythonVersion is what the fake venv interpreter answers: inside the
// bounds of the PyPI tools.
const venvPythonVersion = "Python 3.12.1"

// exeSuffix is what Windows adds to a fake that runnertest.Install names.
const exeSuffix = ".exe"

// Install puts a fake of t at its layout path in dir with the receipt a
// real install writes, and answers the path.
func Install(tb testing.TB, dir string, t toolchain.Tool, goos, goarch string, b runnertest.Behavior) string {
	tb.Helper()
	path := Unreceipted(tb, dir, t, goos, goarch, b)
	data, err := os.ReadFile(path) //nolint:gosec // the fake this helper just installed
	if err != nil {
		tb.Fatalf("read the fake %s: %v", path, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		tb.Fatalf("stat the fake %s: %v", path, err)
	}
	sum := sha256.Sum256(data)
	receipt := toolchain.Receipt{
		Name: t.Name, Version: t.Version, Path: path,
		SHA256: hex.EncodeToString(sum[:]), Size: info.Size(), ModTime: info.ModTime(),
	}
	if err := toolchain.WriteReceipt(dir, receipt); err != nil {
		tb.Fatalf("write the receipt of %s: %v", t.Name, err)
	}
	return path
}

// Unreceipted puts a fake of t at its layout path in dir without a
// receipt: what a copy by hand leaves. A nil probe answers the pin.
func Unreceipted(tb testing.TB, dir string, t toolchain.Tool, goos, goarch string, b runnertest.Behavior) string {
	tb.Helper()
	want := toolchain.ExecPath(dir, t, goos, goarch)
	if strings.HasSuffix(want, ".cmd") {
		tb.Skip("an npm shim on Windows is a .cmd file, which a fake tool cannot play")
	}
	if b.Probe == nil {
		b.Probe, b.Version = t.Probe, t.Name+" "+t.Version
	}
	if err := os.MkdirAll(filepath.Dir(want), dirMode); err != nil {
		tb.Fatalf("make the directory of %s: %v", t.Name, err)
	}
	var got string
	if t.Script {
		got = script(tb, dir, want, t, b)
	} else {
		got = runnertest.Install(tb, filepath.Dir(want), strings.TrimSuffix(filepath.Base(want), exeSuffix), b)
	}
	if got != want {
		tb.Fatalf("the fake of %s landed at %s, the layout names %s", t.Name, got, want)
	}
	if t.Distribution == toolchain.DistPyPI {
		venvPython(tb, dir, goos)
	}
	return got
}

// script plays a PyPI tool that ships a plain #! script, as pip leaves
// checkov: the script at the layout path names a fake interpreter, kept at
// the top of dir so the #! line stays inside the kernel's bound.
func script(tb testing.TB, dir, want string, t toolchain.Tool, b runnertest.Behavior) string {
	tb.Helper()
	b.Script = true
	interpreters := filepath.Join(dir, "interpreters")
	if err := os.MkdirAll(interpreters, dirMode); err != nil {
		tb.Fatalf("make the interpreter directory of %s: %v", t.Name, err)
	}
	interpreter := runnertest.Install(tb, interpreters, t.Bin, b)
	if err := os.WriteFile(want, []byte("#!"+interpreter+"\n"), scriptMode); err != nil {
		tb.Fatalf("write the script of %s: %v", t.Name, err)
	}
	return want
}

// venvPython puts the interpreter a real venv carries beside its tools, so
// a PyPI fake resolves as one a setup run left.
func venvPython(tb testing.TB, dir, goos string) {
	tb.Helper()
	python := toolchain.VenvPython(dir, goos)
	// A test that already placed its own interpreter keeps its behavior.
	_, err := os.Stat(python)
	if err == nil {
		return
	}
	runnertest.Install(tb, filepath.Dir(python), strings.TrimSuffix(filepath.Base(python), exeSuffix), runnertest.Behavior{Probe: []string{"--version"}, Version: venvPythonVersion})
}

// OnPath puts a fake of t in dir under the executable name PATH finds it
// by, answering version to its probe, with no receipt anywhere.
func OnPath(tb testing.TB, dir string, t toolchain.Tool, version string, b runnertest.Behavior) string {
	tb.Helper()
	name := t.Bin
	if name == "" {
		name = t.Name
	}
	b.Probe, b.Version = t.Probe, version
	return runnertest.Install(tb, dir, name, b)
}

// InstallFor installs, with their receipts, a fake of every pinned tool
// the languages need, as setup --surface would for a repository holding
// them, and answers the paths by tool name.
func InstallFor(tb testing.TB, dir string, langs []surface.Language, goos, goarch string, b runnertest.Behavior) map[string]string {
	tb.Helper()
	paths := make(map[string]string)
	tools := toolchain.ForLanguages(langs)
	for i := range tools {
		paths[tools[i].Name] = Install(tb, dir, tools[i], goos, goarch, b)
	}
	return paths
}

// Calls answers the argument lists the fake at path was started with, in
// order, whatever executable suffix the platform gave it.
func Calls(tb testing.TB, path string) [][]string {
	tb.Helper()
	// A plain script fake records through the interpreter its #! line names.
	if interpreter, found := shebang(path); found {
		path = interpreter
	}
	return runnertest.Calls(tb, filepath.Dir(path), strings.TrimSuffix(filepath.Base(path), exeSuffix))
}

// shebang answers the interpreter the #! line of the file at path names.
func shebang(path string) (string, bool) {
	f, err := os.Open(path) //nolint:gosec // the fake its test installed
	if err != nil {
		return "", false
	}
	defer func() { _ = f.Close() }()
	line, err := bufio.NewReader(io.LimitReader(f, maxShebang)).ReadString('\n')
	rest, found := strings.CutPrefix(line, "#!")
	return strings.TrimSpace(rest), err == nil && found
}
