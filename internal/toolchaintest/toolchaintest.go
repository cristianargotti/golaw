// Package toolchaintest installs fake pinned tools into a temporary tool
// directory at the path the layout names, with the receipt a real install
// writes, so the resolver, the lint dispatch, doctor and the hook see an
// installed tool without the network.
package toolchaintest

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// dirMode keeps the fake's directory the developer's own, as setup does.
const dirMode = 0o700

// venvPythonVersion is what the fake venv interpreter answers: inside the
// bounds of the PyPI tools.
const venvPythonVersion = "Python 3.12.1"

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
	got := runnertest.Install(tb, filepath.Dir(want), strings.TrimSuffix(filepath.Base(want), ".exe"), b)
	if got != want {
		tb.Fatalf("the fake of %s landed at %s, the layout names %s", t.Name, got, want)
	}
	if t.Distribution == toolchain.DistPyPI {
		venvPython(tb, dir, goos)
	}
	return got
}

// venvPython puts the interpreter a real venv carries beside its tools, so
// a PyPI fake resolves as one a setup run left.
func venvPython(tb testing.TB, dir, goos string) {
	tb.Helper()
	python := toolchain.VenvPython(dir, goos)
	if _, err := os.Stat(python); err == nil {
		return
	}
	runnertest.Install(tb, filepath.Dir(python), strings.TrimSuffix(filepath.Base(python), ".exe"), runnertest.Behavior{Probe: []string{"--version"}, Version: venvPythonVersion})
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
	return runnertest.Calls(tb, filepath.Dir(path), strings.TrimSuffix(filepath.Base(path), ".exe"))
}
