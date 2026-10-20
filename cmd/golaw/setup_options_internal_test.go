package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/buildinfo"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/setup"
)

func TestParseSetupBinDirectoryRejectsAmbiguousLocations(t *testing.T) {
	t.Parallel()
	b, home := testBinding(t)
	dir := filepath.Join(t.TempDir(), "tools with spaces")
	for _, tc := range []struct{ name, output, want string }{
		{"empty", "", ""},
		{"one line", dir + "\n", ""},
		{"extra line", dir + "\n\nextra\n", ""},
		{"empty paths", "\n\n", filepath.Join(home, "go", "bin")},
		{"empty first GOPATH", "\n" + string(os.PathListSeparator) + dir + "\n", ""},
		{"relative GOBIN", "bin\n" + dir + "\n", ""},
		{"relative GOPATH", "\nworkspace\n", ""},
		{"GOBIN spaces and CRLF", dir + "\r\n\r\n", dir},
		{"GOPATH spaces and CRLF", "\r\n" + dir + "\r\n", filepath.Join(dir, "bin")},
		{"two lines without terminator", dir + "\nunused", dir},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := b.parseSetupBinDirectory(tc.output)
			if tc.want == "" {
				if got != "" || err == nil || errors.Is(err, model.ErrUsage) ||
					!strings.Contains(err.Error(), "run golaw setup again") {
					t.Fatalf("ambiguous directory accepted: %q, %v", got, err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("directory = %q, %v, want %q", got, err, tc.want)
			}
		})
	}
}

//nolint:paralleltest // the fake go is found through PATH
func TestSetupOptionsPreserveTheRepositoryContext(t *testing.T) {
	bin := t.TempDir()
	fakeGo(t, bin+"\n\n")
	root := repository(t, true)
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	b := bindApp(t).bind()
	env := []string{"PATH=" + bin, "GIT_DIR=relative-git", "GIT_INDEX_FILE=relative-index"}
	b.environ = func() []string { return env }
	dirs, err := b.dirs()
	if err != nil {
		t.Fatal(err)
	}
	instant := now().Add(time.Minute)
	options, err := b.setupOptions(t.Context(), nested, instant)
	if err != nil {
		t.Fatal(err)
	}
	if options.Root != root || options.StateDir != dirs.State || options.Version != testVersion ||
		options.BinDir != bin || !options.Now.Equal(instant) || !slices.Equal(options.Env, env) {
		t.Fatalf("setup context = %+v", options)
	}
}

//nolint:paralleltest // the fake go is found through PATH
func TestSetupOptionsPreserveDevelopmentVersionBehavior(t *testing.T) {
	bin := t.TempDir()
	fakeGo(t, bin+"\n\n")
	root := repository(t, true)
	for name, info := range map[string]buildinfo.Info{
		"development": {Version: buildinfo.DevelVersion},
		"pseudo":      {Version: "0.0.0-20261015004959-dd849024b474", Stamped: true},
		"unstamped":   {Version: "1.0.0"},
	} {
		t.Run(name, func(t *testing.T) {
			b := bindApp(t).bind()
			b.info = info
			options, err := b.setupOptions(t.Context(), root, now())
			if err != nil || options.Version != info.Version || options.BinDir != bin {
				t.Fatalf("development options = %+v, %v", options, err)
			}
		})
	}
}

func TestSetupOptionsExplainGoEnvironmentFailure(t *testing.T) {
	root := repository(t, true)
	bin := t.TempDir()
	runnertest.Install(t, bin, "go", runnertest.Behavior{
		Probe: []string{"version"}, Stderr: "configuration rejected\n", Code: 2,
	})
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	options, err := bindApp(t).bind().setupOptions(t.Context(), root, now())
	if options.BinDir != "" || err == nil || errors.Is(err, model.ErrUsage) ||
		!strings.Contains(err.Error(), "exited 2: configuration rejected") ||
		!strings.Contains(err.Error(), "repair the go environment, then run golaw setup again") {
		t.Fatalf("go environment failure = %+v, %v", options, err)
	}
}

func TestSetupBinDirectoryExplainsMissingGo(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	got, err := bindApp(t).bind().setupBinDirectory(t.Context(), setup.Options{Root: t.TempDir()})
	if got != "" || !errors.Is(err, runner.ErrStart) || errors.Is(err, model.ErrUsage) ||
		!strings.Contains(err.Error(), "put go on PATH, then run golaw setup again") {
		t.Fatalf("missing go = %q, %v", got, err)
	}
}

func TestSetupBinDirectoryOnlyNeedsHomeForTheDefault(t *testing.T) {
	t.Parallel()
	b := bindApp(t).bind()
	b.home = func() (string, error) { return "", os.ErrNotExist }
	got, err := b.parseSetupBinDirectory("\n\n")
	if got != "" || !errors.Is(err, os.ErrNotExist) || errors.Is(err, model.ErrUsage) ||
		!strings.Contains(err.Error(), "set GOBIN, then run golaw setup again") {
		t.Fatalf("home lookup = %q, %v", got, err)
	}
	ta := bindApp(t)
	if code := ta.exitCode(err); code != exitFailure {
		t.Fatalf("home lookup exit = %d", code)
	}
	dir := t.TempDir()
	for _, tc := range []struct{ output, want string }{
		{dir + "\n\n", dir},
		{"\n" + dir + "\n", filepath.Join(dir, "bin")},
	} {
		if path, pathErr := b.parseSetupBinDirectory(tc.output); pathErr != nil || path != tc.want {
			t.Fatalf("configured bin directory = %q, %v, want %q", path, pathErr, tc.want)
		}
	}
}
