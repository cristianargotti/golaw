package policy

import (
	"fmt"
	"path/filepath"

	"github.com/cristianargotti/golaw/internal/model"
)

// appDir is the segment every kit directory ends with.
const appDir = "golaw"

// Dirs are the per-OS directories outside the checkout: State holds the
// enrollment records, Config the installed profile bundles and Tools the
// pinned surface linters of M6.5.2, empty when the environment names no
// directory for them.
type Dirs struct {
	State  string
	Config string
	Tools  string
}

// DefaultDirs resolves the directories for an operating system from the
// environment the caller reads: XDG_STATE_HOME and XDG_CONFIG_HOME when
// set, the local and roaming application data on Windows, and
// ~/.local/state and ~/.config elsewhere, each with a golaw segment. The
// package never reads the environment itself; cmd hands it os.Getenv.
func DefaultDirs(goos string, env func(string) string) (Dirs, error) {
	state, err := baseDir(goos, env, "XDG_STATE_HOME", "LOCALAPPDATA", filepath.Join(".local", "state"))
	if err != nil {
		return Dirs{}, err
	}
	config, err := ConfigDir(goos, env)
	if err != nil {
		return Dirs{}, err
	}
	// The tools directory is optional: an environment without one keeps its
	// state and config directories, and the toolchain names the cause.
	tools, err := ToolsDir(goos, env)
	if err != nil {
		tools = ""
	}
	return Dirs{State: filepath.Join(state, appDir), Config: config, Tools: tools}, nil
}

// ToolsDir is the directory of the pinned surface linters (M6.5.2):
// XDG_DATA_HOME when set, the local application data on Windows and
// ~/.local/share elsewhere, macOS included, each with golaw/tools.
func ToolsDir(goos string, env func(string) string) (string, error) {
	data, err := baseDir(goos, env, "XDG_DATA_HOME", "LOCALAPPDATA", filepath.Join(".local", "share"))
	if err != nil {
		return "", err
	}
	return filepath.Join(data, appDir, "tools"), nil
}

// ConfigDir is the config directory of DefaultDirs alone, for a load that
// reads no enrollment record and so has no use for the state directory.
func ConfigDir(goos string, env func(string) string) (string, error) {
	config, err := baseDir(goos, env, "XDG_CONFIG_HOME", "APPDATA", ".config")
	if err != nil {
		return "", err
	}
	return filepath.Join(config, appDir), nil
}

// baseDir picks the first set variable, then the Windows variable on
// Windows, then the home relative fallback.
func baseDir(goos string, env func(string) string, xdg, windows, fallback string) (string, error) {
	if dir := env(xdg); dir != "" {
		return dir, nil
	}
	if goos == "windows" {
		if dir := env(windows); dir != "" {
			return dir, nil
		}
		return "", fmt.Errorf("%w: neither %s nor %s is set", model.ErrUsage, xdg, windows)
	}
	home := env("HOME")
	if home == "" {
		return "", fmt.Errorf("%w: neither %s nor HOME is set", model.ErrUsage, xdg)
	}
	return filepath.Join(home, fallback), nil
}
