package install

import (
	"encoding/base64"
	"encoding/binary"
	"strings"
	"unicode/utf16"
)

// The script of the encoded launcher, around its two string literals: a
// process started with inherited standard handles, so the bytes the host
// wrote reach the binary as they are, where a text pipeline of PowerShell
// would decode them, write them again in its own encoding and add a line
// break. The return value of Start is discarded, because anything a
// script prints would reach the host as part of the hook's answer.
const (
	scriptOpen  = "$process = New-Object System.Diagnostics.Process\n$process.StartInfo.FileName = "
	scriptArgs  = "\n$process.StartInfo.Arguments = "
	scriptClose = "\n$process.StartInfo.UseShellExecute = $false\n" +
		"$null = $process.Start()\n$process.WaitForExit()\n" +
		"$status = $process.ExitCode\n$process.Dispose()\nexit $status\n"
)

func launcherScript(executable, arguments string) string {
	return scriptOpen + powershellLiteral(executable) + scriptArgs + powershellLiteral(arguments) + scriptClose
}

// powershellLiteral writes a single-quoted PowerShell string, the one
// form that expands nothing, with each apostrophe doubled.
func powershellLiteral(text string) string {
	return "'" + strings.ReplaceAll(text, "'", "''") + "'"
}

// encodedLauncher encodes the script as powershell.exe -EncodedCommand
// reads it: standard base64 of its UTF-16LE bytes.
func encodedLauncher(executable, arguments string) string {
	units := utf16.Encode([]rune(launcherScript(executable, arguments)))
	data := make([]byte, 2*len(units))
	for index, unit := range units {
		binary.LittleEndian.PutUint16(data[2*index:], unit)
	}
	return base64.StdEncoding.EncodeToString(data)
}

// crtJoin writes the arguments as the one command line the C runtime of
// the child splits back into the same words.
func crtJoin(args []string) string {
	quoted := make([]string, len(args))
	for index, arg := range args {
		quoted[index] = crtQuote(arg)
	}
	return strings.Join(quoted, " ")
}

// crtQuote quotes one argument for the C runtime: an empty argument, a
// space, a tab or a quote needs the quotes, a quote inside them is
// escaped, and the backslashes before a quote or before the closing
// quote are doubled, because the runtime reads them in pairs there.
func crtQuote(arg string) string {
	if arg != "" && !strings.ContainsAny(arg, " \t\"") {
		return arg
	}
	parts := []string{`"`}
	backslashes := 0
	for _, r := range arg {
		switch r {
		case '\\':
			backslashes++
			continue
		case '"':
			parts = append(parts, strings.Repeat(`\`, 2*backslashes+1), `"`)
		default:
			parts = append(parts, strings.Repeat(`\`, backslashes), string(r))
		}
		backslashes = 0
	}
	parts = append(parts, strings.Repeat(`\`, 2*backslashes), `"`)
	return strings.Join(parts, "")
}
