package lint

import (
	"io/fs"
	"path/filepath"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/surface"
)

func pythonDriver(t *testing.T, format Format) Driver {
	t.Helper()
	for _, d := range pythonDrivers() {
		if d.Format == format {
			return d
		}
	}
	t.Fatalf("no python driver answers %s", format)
	return Driver{}
}

func TestPythonDriversServePythonFiles(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		format Format
		tool   string
	}{
		"ruff check":        {format: FormatRuff, tool: pythonRuff},
		"ruff format check": {format: FormatRuffFormat, tool: pythonRuff},
		"pyright":           {format: FormatPyright, tool: pythonPyright},
	}
	if got := len(pythonDrivers()); got != len(tests) {
		t.Fatalf("pythonDrivers() holds %d drivers, want %d", got, len(tests))
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			d := pythonDriver(t, tc.format)
			if d.Tool != tc.tool || !slices.Equal(d.Languages, []surface.Language{surface.Python}) {
				t.Errorf("driver = %s over %v, want %s over python", d.Tool, d.Languages, tc.tool)
			}
			for _, set := range []string{model.SurfaceParity, model.SurfaceStrict} {
				if _, err := fs.Stat(d.Config, set+"/"+pythonRuffConfig); err != nil {
					t.Errorf("driver config lacks %s/%s: %v", set, pythonRuffConfig, err)
				}
			}
		})
	}
}

func TestPythonDriverArguments(t *testing.T) {
	t.Parallel()
	root, cfg := "repo", filepath.Join("tools", "config")
	files := []string{"deploy/install.py", "test/deploy/test_install.py"}
	abs := []string{filepath.Join(root, "deploy", "install.py"), filepath.Join(root, "test", "deploy", "test_install.py")}
	ruffCfg := filepath.Join(cfg, pythonRuffConfig)
	check := append([]string{"check", "--no-cache", "--config", ruffCfg, "--output-format", "json"}, abs...)
	tests := map[string]struct {
		format Format
		in     Input
		want   []string
	}{
		"ruff check under parity": {format: FormatRuff, in: Input{Root: root, Files: files, ConfigDir: cfg, Set: model.SurfaceParity}, want: check},
		"ruff check under strict": {format: FormatRuff, in: Input{Root: root, Files: files, ConfigDir: cfg, Set: model.SurfaceStrict}, want: check},
		"ruff check with no file (near miss: never the working directory)": {
			format: FormatRuff, in: Input{Root: root, ConfigDir: cfg, Set: model.SurfaceStrict}, want: nil,
		},
		"ruff format under strict": {
			format: FormatRuffFormat, in: Input{Root: root, Files: files, ConfigDir: cfg, Set: model.SurfaceStrict},
			want: append([]string{"format", "--no-cache", "--config", ruffCfg, "--check", "--output-format", "concise"}, abs...),
		},
		"ruff format under parity (near miss: strict only)": {
			format: FormatRuffFormat, in: Input{Root: root, Files: files, ConfigDir: cfg, Set: model.SurfaceParity}, want: nil,
		},
		"pyright under parity": {
			format: FormatPyright, in: Input{Root: root, Files: files, ConfigDir: cfg, Set: model.SurfaceParity},
			want: append([]string{"--outputjson", "--project", filepath.Join(cfg, pythonPyrightConfig)}, abs...),
		},
		"pyright with no file (near miss: never the project directory)": {
			format: FormatPyright, in: Input{Root: root, ConfigDir: cfg, Set: model.SurfaceParity}, want: nil,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := pythonDriver(t, tc.format).Args(tc.in); !slices.Equal(got, tc.want) {
				t.Errorf("Args = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPythonHookRunsRuffCheckOnTheBytes(t *testing.T) {
	t.Parallel()
	cfg := filepath.Join("tools", "config")
	got := pythonDriver(t, FormatRuff).HookArgs(HookInput{Path: "test/deploy/test_install.py", ConfigDir: cfg})
	want := []string{
		"check", "--no-cache", "--config", filepath.Join(cfg, pythonRuffConfig),
		"--output-format", "json", "--stdin-filename", "test/deploy/test_install.py", "-",
	}
	if !slices.Equal(got, want) {
		t.Errorf("HookArgs = %q, want %q", got, want)
	}
	for _, format := range []Format{FormatRuffFormat, FormatPyright} {
		if pythonDriver(t, format).HookArgs != nil {
			t.Errorf("%s runs in the hook; only ruff check may (M6.6.1)", format)
		}
	}
}
