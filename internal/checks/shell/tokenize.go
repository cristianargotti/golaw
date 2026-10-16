package shell

import "strings"

// segment is one simple command, its words with quotes resolved and its
// redirections apart, or one redirection of a command (redirect.go).
type segment []string

// maxDepth bounds the unwrapping of shells that run a script argument.
const maxDepth = 8

// separators end a segment: command lists, pipelines, subshells and lines.
const separators = ";&|()\n\r"

// dialect is how a shell reads its command line, named by the rune that
// escapes the next one outside quotes and inside double quotes.
type dialect rune

// The dialects of the tools and the wrappers: a POSIX shell escapes with a
// backslash, PowerShell with a backtick, where the backslash is the path
// separator, and cmd is read apart (splitCmd), by its own rules and by the
// Microsoft argument rules of the program it starts.
const (
	posix      dialect = '\\'
	powerShell dialect = '`'
	cmdShell   dialect = 0
)

// cmdProgram is the program name of cmd, which reads its script apart.
const cmdProgram = "cmd"

// segments splits a command line into simple commands the way the tool's
// shell reads it: quotes and escapes are resolved, the separators end a
// segment, and a shell that runs a script argument (sh -c, bash -lc, pwsh
// -Command, cmd /c) contributes the segments of that script, read in its
// own dialect, in its place, behind the same assignments and wrappers a
// git call is seen through (env bash -lc, FOO=1 sh -c). Expansions stay
// verbatim: the rules match words, they never run the shell.
func segments(command, tool string) []segment {
	if strings.EqualFold(tool, "PowerShell") {
		return split(command, powerShell, 0)
	}
	return split(command, posix, 0)
}

func split(command string, d dialect, depth int) []segment {
	var segs []segment
	var scripts []string
	if d == cmdShell {
		segs = splitCmd(command)
	} else {
		s := scan(command, d)
		segs, scripts = s.out, s.substitutions()
	}
	var out []segment
	for _, seg := range segs {
		out = append(out, unfold(seg, depth)...)
	}
	if depth < maxDepth {
		for _, script := range scripts {
			out = append(out, split(script, d, depth+1)...)
		}
	}
	return out
}

// unfold answers the segments one segment contributes: itself or, when its
// program is a shell, an eval or an Invoke-Expression that runs a script,
// the words before the script, which the wrappers and the shell read as
// their own options and the name rules judge, then the segments of the
// script, read in its own dialect.
func unfold(seg segment, depth int) []segment {
	words, at := unwrapped(seg)
	script, d, from, ok := shellScript(segment(words[at:]))
	if !ok || depth >= maxDepth {
		return []segment{seg}
	}
	return append([]segment{segment(words[:at+from])}, split(script, d, depth+1)...)
}

// base is the program name of a word: its last path element after a slash
// or a backslash, lower case, without the .exe suffix, so C:\Git\cmd\git.exe
// on Windows is the same program as git for every rule that compares the
// name.
func base(word string) string {
	return strings.TrimSuffix(strings.ToLower(word[strings.LastIndexAny(word, `/\`)+1:]), ".exe")
}

// isShell reports whether a program runs a script given as an argument.
func isShell(name string) bool {
	switch base(name) {
	case "sh", "bash", "zsh", "dash", "ksh", "fish", "pwsh", "powershell", cmdProgram:
		return true
	default:
		return false
	}
}

// shellScript returns the script a shell segment runs, the words after -c,
// -lc, -Command or /c, with the dialect that shell reads it in and the
// index of its first word, and the script an eval or an Invoke-Expression
// reads again (evalScript).
func shellScript(seg segment) (string, dialect, int, bool) {
	if script, d, at, ok := evalScript(seg); ok {
		return script, d, at, true
	}
	if len(seg) < 2 || !isShell(seg[0]) {
		return "", posix, 0, false
	}
	for i := 1; i+1 < len(seg); i++ {
		switch strings.ToLower(seg[i]) {
		case "-c", "-lc", "-ec", "-command", "/c":
			return strings.Join(seg[i+1:], " "), dialectOf(seg[0]), i + 1, true
		}
	}
	return "", posix, 0, false
}

// evalScript returns the words of an eval joined by spaces, which the
// POSIX shell reads again, and the string Invoke-Expression or its alias
// iex runs as PowerShell, after its -Command parameter or an abbreviation
// of it, each with the index of its first word.
func evalScript(seg segment) (string, dialect, int, bool) {
	switch {
	case len(seg) < 2:
		return "", posix, 0, false
	case seg[0] == "eval":
		return strings.Join(seg[1:], " "), posix, 1, true
	case strings.EqualFold(seg[0], "Invoke-Expression") || strings.EqualFold(seg[0], "iex"):
		at := 1
		if len(seg) > 2 && abbreviates(seg[1], "-command") {
			at = 2
		}
		return strings.Join(seg[at:], " "), powerShell, at, true
	default:
		return "", posix, 0, false
	}
}

// abbreviates reports whether a word names a PowerShell parameter by a
// prefix of its name, which PowerShell accepts.
func abbreviates(word, parameter string) bool {
	return len(word) > 1 && strings.HasPrefix(parameter, strings.ToLower(word))
}

// dialectOf is the dialect of a shell program.
func dialectOf(program string) dialect {
	switch base(program) {
	case "pwsh", "powershell":
		return powerShell
	case cmdProgram:
		return cmdShell
	default:
		return posix
	}
}
