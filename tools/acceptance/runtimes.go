package main

import (
	"context"
	"fmt"
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
			if err := linkRuntime(dir, links, runtimeLinks(rt.Name), runtime.GOOS); err != nil {
				return nil, err
			}
			linked = true
			break
		}
	}
	if !linked {
		return nil, nil
	}
	return []string{links}, nil
}

// linkRuntime puts in links each program of names that dir holds: a
// symbolic link, or on Windows a command shim, which needs no privilege
// and keeps the program beside the libraries it loads.
func linkRuntime(dir, links string, names []string, goos string) error {
	if err := os.MkdirAll(links, dirMode); err != nil {
		return fmt.Errorf("create the runtime links: %w", err)
	}
	for _, name := range names {
		target, found := runtimeProgram(dir, name, goos)
		if !found {
			continue
		}
		var err error
		if goos == goosWindows {
			err = os.WriteFile(filepath.Join(links, name+".cmd"), []byte("@\""+target+"\" %*\r\n"), fileMode)
		} else {
			err = os.Symlink(target, filepath.Join(links, name))
		}
		if err != nil && !os.IsExist(err) {
			return fmt.Errorf("link %s: %w", name, err)
		}
	}
	return nil
}

// runtimeProgram is the file dir holds for name, with the extension
// Windows starts it by.
func runtimeProgram(dir, name, goos string) (string, bool) {
	candidates := []string{name}
	if goos == goosWindows {
		candidates = []string{name + ".exe", name + ".cmd", name + ".bat"}
	}
	for _, candidate := range candidates {
		path := filepath.Join(dir, candidate)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, true
		}
	}
	return "", false
}
