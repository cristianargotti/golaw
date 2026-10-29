package hometest_test

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hometest"
	"github.com/cristianargotti/golaw/internal/policy"
)

func TestNewIsolatesEveryDirectoryUnderTheTestTree(t *testing.T) {
	t.Parallel()
	h := hometest.New(t)
	for name, dir := range map[string]string{"home": h.Dir, "state": h.State, "config": h.Config, "data": h.Data} {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			t.Errorf("%s %q is not a directory: %v", name, dir, err)
		}
	}
	if hometest.New(t).Dir == h.Dir {
		t.Error("two homes of one test share a directory")
	}
}

func TestGetenvAnswersTheIsolatedVariablesAndNothingElse(t *testing.T) {
	t.Parallel()
	h := hometest.New(t)
	tests := map[string]struct {
		name string
		want string
	}{
		"HOME":                                   {name: "HOME", want: h.Dir},
		"USERPROFILE":                            {name: "USERPROFILE", want: h.Dir},
		"XDG_STATE_HOME":                         {name: "XDG_STATE_HOME", want: h.State},
		"LOCALAPPDATA":                           {name: "LOCALAPPDATA", want: h.State},
		"XDG_CONFIG_HOME":                        {name: "XDG_CONFIG_HOME", want: h.Config},
		"APPDATA":                                {name: "APPDATA", want: h.Config},
		"XDG_DATA_HOME":                          {name: "XDG_DATA_HOME", want: h.Data},
		"PATH, which the test names (near miss)": {name: "PATH", want: ""},
		"any other name (near miss)":             {name: "GOLAW_TOOLS_DIR", want: ""},
	}
	for label, tc := range tests {
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			if got := h.Getenv(tc.name); got != tc.want {
				t.Errorf("Getenv(%s) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

func TestEnvironIsTheSortedListOfTheSameVariables(t *testing.T) {
	t.Parallel()
	h := hometest.New(t)
	environ := h.Environ()
	if !slices.IsSorted(environ) {
		t.Errorf("Environ() = %v, want sorted", environ)
	}
	for _, line := range environ {
		name, value, ok := strings.Cut(line, "=")
		if !ok || h.Getenv(name) != value {
			t.Errorf("Environ() line %q disagrees with Getenv", line)
		}
	}
	if !slices.Contains(environ, "HOME="+h.Dir) {
		t.Errorf("Environ() = %v, want HOME", environ)
	}
}

func TestTheDirectoriesArePolicyDirectories(t *testing.T) {
	t.Parallel()
	h := hometest.New(t)
	dirs, err := policy.DefaultDirs(runtime.GOOS, h.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	want := policy.Dirs{State: filepath.Join(h.State, "golaw"), Config: filepath.Join(h.Config, "golaw"), Tools: h.Tools()}
	if dirs != want {
		t.Errorf("DefaultDirs = %+v, want %+v", dirs, want)
	}
	if h.Tools() != filepath.Join(h.Data, "golaw", "tools") {
		t.Errorf("Tools() = %q", h.Tools())
	}
}
