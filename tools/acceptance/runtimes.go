package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"github.com/cristianargotti/golaw/internal/toolchain"
)

// runtimeLinksDir is the directory of a run or a case that holds the links.
const runtimeLinksDir = "runtime-links"

// runtimeLinks names the programs a runtime brings that setup and the
// tools start by name: node and npm, python3 and python.
func runtimeLinks(name string) []string {
	if name == "node" {
		return []string{"node", "npm"}
	}
	return []string{"python3", "python"}
}

// surfaceRuntimes finds the first directory of the caller's PATH whose node,
// and then whose python3, answers within the bounds of the kit's toolchain
// table: setup installs the npm and PyPI tools on the person's own runtimes,
// the kit never installs either one, and a version manager shim or an old
// system Python ahead of a usable one is passed over. It links those
// programs alone into links and answers that directory, never the
// runtimes' own: a system directory beside python3 also holds an older go
// that would shadow the caller's. A runtime the caller lacks has no link,
// so setup reports it as missing.
func (s *suite) surfaceRuntimes(ctx context.Context, links string) ([]string, error) {
	linked := false
	for _, rt := range toolchain.Runtimes() {
		for _, dir := range filepath.SplitList(environmentValue(s.env, envPath)) {
			probe := toolchain.Resolver{GOOS: runtime.GOOS, Env: []string{pathAssignment + dir}}
			if dir == "" || !probe.Runtime(ctx, rt).OK {
				continue
			}
			if err := linkRuntime(dir, links, runtimeLinks(rt.Name)); err != nil {
				return nil, err
			}
			linked = true
			break
		}
	}
	if !linked {
		return nil, nil
	}
	// cmd.exe starts a shim by the path PATH names and splits it at a
	// space, as the run's own directory holds one by design.
	return []string{shortPath(links)}, nil
}

// linkRuntime puts in links each program of names that dir holds: a
// symbolic link, or on Windows a command shim, which needs no privilege
// and keeps the program beside the libraries it loads.
func linkRuntime(dir, links string, names []string) error {
	if err := os.MkdirAll(links, dirMode); err != nil {
		return fmt.Errorf("create the runtime links: %w", err)
	}
	for _, name := range names {
		target, found := runtimeProgram(dir, name)
		if !found {
			continue
		}
		if err := linkProgram(target, filepath.Join(links, name)); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
	}
	return nil
}

// writeShim writes link.cmd, the Windows link, confined to the links
// directory through an os.Root, so the shim never lands outside the
// directory PATH names. It starts target quoted by its short path:
// cmd.exe splits a path at a space and reads a batch file in the console
// code page, never UTF-8.
func writeShim(link, target string) error {
	root, err := os.OpenRoot(filepath.Dir(link))
	if err != nil {
		return fmt.Errorf("open the runtime links: %w", err)
	}
	err = errors.Join(root.WriteFile(filepath.Base(link)+".cmd", []byte("@\""+shortPath(target)+"\" %*\r\n"), fileMode), root.Close())
	if err != nil {
		return fmt.Errorf("shim %s: %w", filepath.Base(link), err)
	}
	return nil
}

// runtimeProgram is the file dir holds for name, with the extension
// Windows starts it by.
func runtimeProgram(dir, name string) (string, bool) {
	for _, candidate := range programCandidates(name, runtime.GOOS == goosWindows) {
		path := filepath.Join(dir, candidate)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, true
		}
	}
	return "", false
}

// programCandidates are the file names a program goes by.
func programCandidates(name string, windows bool) []string {
	if windows {
		return []string{name + ".exe", name + ".cmd", name + ".bat"}
	}
	return []string{name}
}
