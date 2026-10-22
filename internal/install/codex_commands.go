package install

import "fmt"

// codexCommands writes the hook commands of a Codex installation: the
// POSIX command every system gets and, for a Windows target only, the
// encoded launcher of commandWindows. Each is decoded again before it is
// handed out, because a hooks file that names another binary would run
// that binary on every tool call without a word from the host.
func codexCommands(options Options) (string, string, error) {
	if options.Executable == "" {
		return "", "", fmt.Errorf("hook executable: %w", ErrPath)
	}
	args := []string{"hook", "codex"}
	command := HookCommand(options.Executable, "linux", args...)
	windows := ""
	if options.GOOS == goosWindows {
		windows = HookCommand(options.Executable, goosWindows, args...)
	}
	for _, written := range []string{command, windows} {
		if written == "" {
			continue
		}
		if binary, ok := CommandBinary(written); !ok || binary != options.Executable {
			return "", "", fmt.Errorf("hook command %q does not name %s: %w", written, options.Executable, ErrPath)
		}
	}
	return command, windows, nil
}
