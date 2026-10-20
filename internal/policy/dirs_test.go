package policy_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

func env(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func TestDefaultDirs(t *testing.T) {
	t.Parallel()
	local, roaming, state, config := `C:\L`, `C:\R`, "/s", "/c"
	cases := map[string]struct {
		goos string
		env  map[string]string
		want policy.Dirs
	}{
		"linux home":  {"linux", map[string]string{"HOME": "/h/u"}, policy.Dirs{State: "/h/u/.local/state/golaw", Config: "/h/u/.config/golaw"}},
		"darwin home": {"darwin", map[string]string{"HOME": "/h/mac"}, policy.Dirs{State: "/h/mac/.local/state/golaw", Config: "/h/mac/.config/golaw"}},
		"xdg":         {"linux", map[string]string{"HOME": "/h/u", "XDG_STATE_HOME": "/s", "XDG_CONFIG_HOME": "/c"}, policy.Dirs{State: "/s/golaw", Config: "/c/golaw"}},
		"windows":     {"windows", map[string]string{"LOCALAPPDATA": local, "APPDATA": roaming}, policy.Dirs{State: filepath.Join(local, "golaw"), Config: filepath.Join(roaming, "golaw")}},
		"windows xdg": {"windows", map[string]string{"XDG_STATE_HOME": state, "XDG_CONFIG_HOME": config}, policy.Dirs{State: filepath.Join(state, "golaw"), Config: filepath.Join(config, "golaw")}},
	}
	for name, c := range cases {
		got, err := policy.DefaultDirs(c.goos, env(c.env))
		want := policy.Dirs{State: filepath.FromSlash(c.want.State), Config: filepath.FromSlash(c.want.Config)}
		if err != nil || got != want {
			t.Errorf("%s: %+v, %v, want %+v", name, got, err, want)
		}
	}
	for name, c := range map[string]struct{ goos string }{"no home": {"linux"}, "no appdata": {"windows"}} {
		if _, err := policy.DefaultDirs(c.goos, env(nil)); !errors.Is(err, model.ErrUsage) {
			t.Errorf("%s: %v, want ErrUsage", name, err)
		}
	}
	if _, err := policy.DefaultDirs("windows", env(map[string]string{"LOCALAPPDATA": `C:\L`})); !errors.Is(err, model.ErrUsage) {
		t.Errorf("windows without APPDATA: %v", err)
	}
}

// TestConfigDirNeedsNoStateDirectory resolves the config directory of
// DefaultDirs where no state directory resolves.
func TestConfigDirNeedsNoStateDirectory(t *testing.T) {
	t.Parallel()
	roaming := `C:\R`
	cases := map[string]struct {
		goos string
		env  map[string]string
		want string
	}{
		"xdg alone":  {"linux", map[string]string{"XDG_CONFIG_HOME": "/c"}, "/c/golaw"},
		"linux home": {"linux", map[string]string{"HOME": "/h/u"}, "/h/u/.config/golaw"},
		"windows":    {"windows", map[string]string{"APPDATA": roaming}, filepath.Join(roaming, "golaw")},
	}
	for name, c := range cases {
		if got, err := policy.ConfigDir(c.goos, env(c.env)); err != nil || got != filepath.FromSlash(c.want) {
			t.Errorf("%s: %q, %v, want %q", name, got, err, c.want)
		}
	}
	if _, err := policy.ConfigDir("linux", env(map[string]string{"XDG_STATE_HOME": "/s"})); !errors.Is(err, model.ErrUsage) || !strings.Contains(err.Error(), "neither XDG_CONFIG_HOME nor HOME is set") {
		t.Errorf("no config variable: %v", err)
	}
}
