package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/cristianargotti/golaw/internal/lint"
)

// The exit codes: written or up to date, a row the README contradicts or a
// stale index, a usage or read failure.
const (
	exitClean    = 0
	exitFindings = 1
	exitFailure  = 2
)

// indexMode is the mode of a committed asset, readable by every checkout.
const indexMode = 0o644

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

// options are the flags of one run.
type options struct {
	pkg, rows, out string
	check          bool
}

// parseOptions reads the flags; -package has no default, since the
// installed package lives under a tools directory only setup knows.
func parseOptions(args []string, stderr io.Writer) (options, error) {
	var o options
	flags := flag.NewFlagSet("sonarmap", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&o.pkg, "package", "", "the installed eslint-plugin-sonarjs directory")
	flags.StringVar(&o.rows, "rows", "internal/lint/rulemap/web.yml", "the public web rule map")
	flags.StringVar(&o.out, "out", "internal/lint/config/web/sonar-index.json", "the committed index")
	flags.BoolVar(&o.check, "check", false, "compare with -out instead of writing it")
	if err := flags.Parse(args); err != nil {
		return options{}, fmt.Errorf("flags: %w", err)
	}
	if o.pkg == "" {
		return options{}, errUsage
	}
	return o, nil
}

// manifest is the part of package.json the command reads.
type manifest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// readFile reads a file of the package or the map, naming it on failure.
func readFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path) //nolint:gosec // The path is a file the developer names on the command line.
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return data, nil
}

// readPackage reads package.json first, so another package is named before
// its README is judged.
func readPackage(dir string) (index, error) {
	path := filepath.Join(dir, "package.json")
	data, err := readFile(path)
	if err != nil {
		return index{}, err
	}
	var pkg manifest
	if jsonErr := json.Unmarshal(data, &pkg); jsonErr != nil {
		return index{}, fmt.Errorf("decode %s: %w", path, jsonErr)
	}
	if pkg.Name != pluginName {
		return index{}, fmt.Errorf("%w: %s names %q", errPackage, path, pkg.Name)
	}
	readme := filepath.Join(dir, "README.md")
	text, err := readFile(readme)
	if err != nil {
		return index{}, err
	}
	idx, err := parseIndex(text, pkg.Version)
	if err != nil {
		return index{}, fmt.Errorf("parse %s: %w", readme, err)
	}
	return idx, nil
}

// load answers the index of the package and the rows of the map.
func load(o options) (index, []lint.Row, error) {
	idx, err := readPackage(o.pkg)
	if err != nil {
		return index{}, nil, err
	}
	data, err := readFile(o.rows)
	if err != nil {
		return index{}, nil, err
	}
	rows, err := lint.DecodeRows(data)
	if err != nil {
		return index{}, nil, fmt.Errorf("decode %s: %w", o.rows, err)
	}
	return idx, rows, nil
}

// run verifies the rows against the package, then writes or checks -out.
func run(args []string, stdout, stderr io.Writer) int {
	o, err := parseOptions(args, stderr)
	if err != nil {
		return failure(stderr, err)
	}
	idx, rows, err := load(o)
	if err != nil {
		return failure(stderr, err)
	}
	if bad := verifyRows(idx, rows); len(bad) > 0 {
		for _, line := range bad {
			if say(stderr, exitFindings, "sonarmap: %s\n", line) != exitFindings {
				return exitFailure
			}
		}
		return exitFindings
	}
	data, err := render(idx)
	if err != nil {
		return failure(stderr, err)
	}
	if o.check {
		return check(o, data, stdout, stderr)
	}
	if err := os.WriteFile(o.out, data, indexMode); err != nil {
		return failure(stderr, err)
	}
	return say(stdout, exitClean, "sonarmap: wrote %s (%d keys)\n", o.out, len(idx.Rules))
}

// check compares the rendering with -out byte for byte; an unreadable file
// is as stale as a different one.
func check(o options, data []byte, stdout, stderr io.Writer) int {
	committed, err := readFile(o.out)
	if err != nil || !bytes.Equal(committed, data) {
		return say(stderr, exitFindings, "sonarmap: %s is stale: run go run ./tools/sonarmap -package %s\n", o.out, o.pkg)
	}
	return say(stdout, exitClean, "sonarmap: %s is up to date\n", o.out)
}

// failure reports an error before the verification.
func failure(stderr io.Writer, err error) int {
	return say(stderr, exitFailure, "sonarmap: %v\n", err)
}

// say prints a line and answers code; a channel that refuses the line
// turns any answer into a failure, since nothing else is left to report on.
func say(w io.Writer, code int, format string, args ...any) int {
	if _, err := fmt.Fprintf(w, format, args...); err != nil {
		return exitFailure
	}
	return code
}
