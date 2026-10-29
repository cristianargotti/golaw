package policy_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

func TestToolsDirFollowsTheDataDirectoryOfEachOS(t *testing.T) {
	t.Parallel()
	local := `C:\L`
	tests := map[string]struct {
		goos string
		env  map[string]string
		want string // empty when the directory cannot be resolved
	}{
		"linux home":                     {goos: "linux", env: map[string]string{"HOME": "/h/u"}, want: "/h/u/.local/share/golaw/tools"},
		"darwin home, as on Linux":       {goos: "darwin", env: map[string]string{"HOME": "/h/mac"}, want: "/h/mac/.local/share/golaw/tools"},
		"xdg data home wins":             {goos: "linux", env: map[string]string{"HOME": "/h/u", "XDG_DATA_HOME": "/d"}, want: "/d/golaw/tools"},
		"windows local application data": {goos: "windows", env: map[string]string{"LOCALAPPDATA": local, "APPDATA": `C:\R`}, want: filepath.Join(local, "golaw", "tools")},
		"no home":                        {goos: "linux", env: nil},
		"windows without local data":     {goos: "windows", env: map[string]string{"APPDATA": `C:\R`}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := policy.ToolsDir(tc.goos, env(tc.env))
			if tc.want == "" {
				if !errors.Is(err, model.ErrUsage) {
					t.Fatalf("ToolsDir = %q, %v, want ErrUsage", got, err)
				}
				return
			}
			if want := filepath.FromSlash(tc.want); err != nil || got != want {
				t.Errorf("ToolsDir = %q, %v, want %q", got, err, want)
			}
		})
	}
}

// TestDefaultDirsNeverFailsForTheToolsDirectory keeps the state and config
// directories of every environment that resolves them today: an
// unresolvable tools directory is empty, and the toolchain then answers
// every tool missing with that cause.
func TestDefaultDirsNeverFailsForTheToolsDirectory(t *testing.T) {
	t.Parallel()
	got, err := policy.DefaultDirs("windows", env(map[string]string{"XDG_STATE_HOME": "/s", "XDG_CONFIG_HOME": "/c"}))
	if err != nil {
		t.Fatalf("DefaultDirs = %v, want the state and config directories", err)
	}
	if got.Tools != "" || got.State == "" || got.Config == "" {
		t.Errorf("dirs = %+v, want an empty tools directory beside the other two", got)
	}
}
