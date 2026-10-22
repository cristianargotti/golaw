package install

import "strings"

// launcherPrefix is the outer invocation of the encoded Windows launcher
// of D13: it carries no shell metacharacter, so cmd /c and PowerShell
// read it alike, and the script travels inside the encoding.
const launcherPrefix = "powershell.exe -NoProfile -NonInteractive -InputFormat Text -OutputFormat Text -EncodedCommand "

// goosWindows is the one target whose hook command is the encoded launcher.
const goosWindows = "windows"

// HookCommand writes the command a host runs for a hook: on Windows the
// encoded PowerShell launcher that starts the executable with the
// arguments, elsewhere the single-quoted executable and the arguments
// for a POSIX shell. The executable string is kept exactly, never
// resolved through a link or read from the environment, so the path a
// package manager links keeps working after an upgrade.
func HookCommand(executable, goos string, args ...string) string {
	if goos == goosWindows {
		return launcherPrefix + encodedLauncher(executable, crtJoin(args))
	}
	return posixCommand(executable, args)
}

// CommandBinary answers the executable a command HookCommand wrote names,
// and false for any other command: the components are parsed and the
// whole command regenerated from them, so a launcher with an altered
// script, a trailing statement or a malformed encoding is never read as
// this kit's.
func CommandBinary(command string) (string, bool) {
	if encoded, found := strings.CutPrefix(command, launcherPrefix); found {
		return launcherBinary(command, encoded)
	}
	words, ok := posixWords(command)
	if !ok || len(words) == 0 || posixCommand(words[0], words[1:]) != command {
		return "", false
	}
	return words[0], true
}

func launcherBinary(command, encoded string) (string, bool) {
	executable, args, ok := decodeLauncher(encoded)
	if !ok || HookCommand(executable, goosWindows, args...) != command {
		return "", false
	}
	return executable, true
}
