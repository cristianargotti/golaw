package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

const (
	envPath        = "PATH"
	pathAssignment = "PATH="
	goosWindows    = "windows"
	dirMode        = 0o700
	fileMode       = 0o600
	execMode       = 0o700
)

func isolatedEnvironment(parent []string, home, bin string) []string {
	env := []string{
		pathAssignment + bin + string(os.PathListSeparator) + environmentValue(parent, envPath),
		"HOME=" + home, "USERPROFILE=" + home,
		"APPDATA=" + filepath.Join(home, "appdata"),
		"LOCALAPPDATA=" + filepath.Join(home, "localappdata"),
		"XDG_CONFIG_HOME=" + filepath.Join(home, "config"),
		"XDG_STATE_HOME=" + filepath.Join(home, "state"),
		"XDG_DATA_HOME=" + filepath.Join(home, "data"),
		"XDG_CACHE_HOME=" + filepath.Join(home, "cache"),
		"CODEX_HOME=" + filepath.Join(home, ".codex"),
		"CLAUDE_CONFIG_DIR=" + filepath.Join(home, ".claude"),
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1",
		"DISABLE_AUTOUPDATER=1", "GOFLAGS=-mod=readonly", "GOTOOLCHAIN=auto",
	}
	for _, key := range []string{
		"SystemRoot", "SystemDrive", "COMSPEC", "PATHEXT",
		"TEMP", "TMP", "GOROOT", "GOCACHE", "GOMODCACHE",
	} {
		if value := environmentValue(parent, key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	return env
}

func environmentValue(env []string, name string) string {
	for _, entry := range env {
		key, value, found := strings.Cut(entry, "=")
		if found && strings.EqualFold(key, name) {
			return value
		}
	}
	return ""
}

func prependPath(env []string, dir string) []string {
	out := slices.Clone(env)
	for i, value := range out {
		key, rest, found := strings.Cut(value, "=")
		if found && strings.EqualFold(key, envPath) {
			out[i] = pathAssignment + dir + string(os.PathListSeparator) + rest
			return out
		}
	}
	return append(out, pathAssignment+dir)
}

func executable(name string) string {
	if runtime.GOOS == goosWindows {
		return name + ".exe"
	}
	return name
}

func writeFile(path string, body []byte, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
		return fmt.Errorf("create acceptance directory: %w", err)
	}
	if err := os.WriteFile(path, body, mode); err != nil {
		return fmt.Errorf("write acceptance file: %w", err)
	}
	return nil
}

func readFile(path string) ([]byte, error) {
	body, err := os.ReadFile(path) //nolint:gosec // The harness reads its source assets and isolated temporary files.
	if err != nil {
		return nil, fmt.Errorf("read acceptance file: %w", err)
	}
	return body, nil
}

func moduleRoot(start string) (string, error) {
	for current := start; ; current = filepath.Dir(current) {
		if info, err := os.Stat(filepath.Join(current, "go.mod")); err == nil && !info.IsDir() {
			return current, nil
		}
		if filepath.Dir(current) == current {
			return "", fmt.Errorf("module root absent: %w", ErrAcceptance)
		}
	}
}
