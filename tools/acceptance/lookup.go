package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// executableBits are the execute permission bits of a file mode.
const executableBits = 0o111

func resolveExecutable(command string, env []string) (string, error) {
	name := command
	if runtime.GOOS == goosWindows && filepath.Ext(name) == "" {
		name += ".exe"
	}
	for _, directory := range filepath.SplitList(environmentValue(env, envPath)) {
		if directory == "" {
			continue
		}
		path := filepath.Join(directory, name)
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		if runtime.GOOS == goosWindows || info.Mode()&executableBits != 0 {
			return path, nil
		}
	}
	return "", fmt.Errorf("installed executable is absent from PATH: %w", ErrAcceptance)
}
