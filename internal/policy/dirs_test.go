package policy_test

import (
	"errors"
	"path/filepath"
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
		"linux home":  {"linux", map[string]string{"HOME": "/home/u"}, policy.Dirs{State: "/home/u/.local/state/golaw", Config: "/home/u/.config/golaw"}},
		"darwin home": {"darwin", map[string]string{"HOME": "/home/mac"}, policy.Dirs{State: "/home/mac/.local/state/golaw", Config: "/home/mac/.config/golaw"}},
		"xdg":         {"linux", map[string]string{"HOME": "/home/u", "XDG_STATE_HOME": "/s", "XDG_CONFIG_HOME": "/c"}, policy.Dirs{State: "/s/golaw", Config: "/c/golaw"}},
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
