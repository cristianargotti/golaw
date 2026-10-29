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
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// dirMode keeps the fake's directory the developer's own, as setup does.
const dirMode = 0o700

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
	return got
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
