package toolchain

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cristianargotti/golaw/internal/runner"
)

// npmArgs install the embedded lock as written: no package script runs on
// the person's machine, and the peer range is the one the lock was
// generated with, which npm ci refuses without the same flag.
func npmArgs() []string {
	return []string{"ci", "--ignore-scripts", "--no-audit", "--no-fund", "--legacy-peer-deps"}
}

// pipArgs install only the hashed wheels of the lock, never a source build
// and never a dependency the lock does not name.
func pipArgs(lock string) []string {
	return []string{"install", "--require-hashes", "--no-deps", "--only-binary", ":all:", "--disable-pip-version-check", "-r", lock}
}

// installNPM runs npm ci over the embedded lock on the person's Node and
// records each tool it placed; Node is a prerequisite, never installed.
func installNPM(ctx context.Context, o InstallOptions, tools []Tool) ([]Receipt, error) {
	resolver := Resolver{Dir: o.Dir, GOOS: o.GOOS, GOARCH: o.GOARCH, Env: o.Env, Run: o.Run}
	if recs, kept, err := allVerified(resolver, tools, o); kept {
		return recs, err
	}
	if err := needRuntime(ctx, resolver, tools[0]); err != nil {
		return nil, err
	}
	npm, found := lookPath("npm", o.Env, o.GOOS)
	if !found {
		return nil, fmt.Errorf("%w: npm is not on PATH beside node", ErrRuntime)
	}
	dir := NodeDir(o.Dir)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return nil, failed(err)
	}
	for _, name := range []string{"package.json", npmLock} {
		if err := placeEmbedded(name, filepath.Join(dir, name)); err != nil {
			return nil, err
		}
	}
	if err := runInstaller(ctx, o, npm, npmArgs(), dir); err != nil {
		return nil, err
	}
	return recordAll(o, tools)
}

// installPyPI builds the kit's virtual environment on the person's Python
// once, then installs the hashed lock into it.
func installPyPI(ctx context.Context, o InstallOptions, tools []Tool) ([]Receipt, error) {
	resolver := Resolver{Dir: o.Dir, GOOS: o.GOOS, GOARCH: o.GOARCH, Env: o.Env, Run: o.Run}
	if recs, kept, err := allVerified(resolver, tools, o); kept {
		return recs, err
	}
	rt, _ := runtimeOf(tools[0])
	python := resolver.Runtime(ctx, rt)
	if !python.OK {
		return nil, fmt.Errorf("%w: %s", ErrRuntime, python.Detail)
	}
	venv := PythonDir(o.Dir)
	home := filepath.Dir(venv)
	if err := os.MkdirAll(home, dirMode); err != nil {
		return nil, failed(err)
	}
	// A venv whose interpreter no longer resolves is rebuilt whole: without
	// --clear, venv keeps the dangling link it finds.
	if _, err := os.Stat(VenvPython(o.Dir, o.GOOS)); err != nil {
		if err := runInstaller(ctx, o, python.Path, []string{"-m", "venv", "--clear", venv}, home); err != nil {
			return nil, err
		}
	}
	lock := filepath.Join(home, pypiLock)
	if err := placeEmbedded(pypiLock, lock); err != nil {
		return nil, err
	}
	if err := runInstaller(ctx, o, venvExec(venv, "pip", o.GOOS), pipArgs(lock), home); err != nil {
		return nil, err
	}
	return recordAll(o, tools)
}

// needRuntime refuses a group whose runtime is absent or out of bounds.
func needRuntime(ctx context.Context, resolver Resolver, t Tool) error {
	rt, _ := runtimeOf(t)
	if status := resolver.Runtime(ctx, rt); !status.OK {
		return fmt.Errorf("%w: %s", ErrRuntime, status.Detail)
	}
	return nil
}

// allVerified answers the receipts of a group whose every tool is already
// in place at its pin, so a second run starts no package manager.
func allVerified(resolver Resolver, tools []Tool, o InstallOptions) ([]Receipt, bool, error) {
	recs := make([]Receipt, 0, len(tools))
	for i := range tools {
		rec, ok := resolver.verified(tools[i])
		if !ok {
			return nil, false, nil
		}
		recs = append(recs, rec)
	}
	var errs []error
	for _, rec := range recs {
		errs = append(errs, say(o.Out, "%s %s já estava instalado e verificado em %s\n", rec.Name, rec.Version, rec.Path))
	}
	return recs, true, errors.Join(errs...)
}

// runInstaller runs one package manager step; its first stderr line names
// what npm or pip refused.
func runInstaller(ctx context.Context, o InstallOptions, name string, args []string, dir string) error {
	out, err := o.Run(ctx, runner.Command{Name: name, Args: args, Dir: dir, Env: o.Env})
	step := filepath.Base(name) + " " + args[0]
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrInstall, step, err)
	}
	if out.Code != 0 {
		return fmt.Errorf("%w: %s exited %d: %s", ErrInstall, step, out.Code, firstLine(runner.Output{Stdout: out.Stderr, Stderr: out.Stdout}))
	}
	return nil
}

func placeEmbedded(name, path string) error {
	data, err := embeddedFile(name)
	if err != nil {
		return failed(err)
	}
	if err := writeAtomic(path, data, fileMode); err != nil {
		return failed(err)
	}
	return nil
}

// recordAll records each tool the package manager placed; one it did not
// place is its own error and never hides the others.
func recordAll(o InstallOptions, tools []Tool) ([]Receipt, error) {
	var recs []Receipt
	var errs []error
	for i := range tools {
		rec, err := record(o, tools[i], ExecPath(o.Dir, tools[i], o.GOOS, o.GOARCH))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		recs = append(recs, rec)
	}
	return recs, errors.Join(errs...)
}

// record writes the receipt of the file at path and tells the person.
func record(o InstallOptions, t Tool, path string) (Receipt, error) {
	rec, err := receiptFor(t, path)
	if err == nil {
		err = WriteReceipt(o.Dir, rec)
	}
	if err != nil {
		return Receipt{}, err
	}
	return rec, say(o.Out, "%s %s instalado e verificado em %s\n", t.Name, t.Version, path)
}

// receiptFor hashes and stats what an installation left at path.
func receiptFor(t Tool, path string) (Receipt, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Receipt{}, fmt.Errorf("%w: %s %s left nothing at %s: %w", ErrInstall, t.Name, t.Version, path, err)
	}
	sum, err := fileSHA256(path)
	if err != nil {
		return Receipt{}, failed(err)
	}
	return Receipt{Name: t.Name, Version: t.Version, Path: path, SHA256: sum, Size: info.Size(), ModTime: info.ModTime()}, nil
}

// failed marks an installation step's error so errors.Is finds ErrInstall.
func failed(err error) error {
	return fmt.Errorf("%w: %w", ErrInstall, err)
}
