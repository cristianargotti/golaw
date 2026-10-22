package install

import (
	"encoding/base64"
	"regexp"
	"strings"
	"testing"
	"unicode/utf16"
)

// cinLauncherScript is the script an encoded launcher command carries.
func cinLauncherScript(t *testing.T, command string) string {
	t.Helper()
	fields := strings.Fields(command)
	data, err := base64.StdEncoding.DecodeString(fields[len(fields)-1])
	must(t, err)
	units := make([]uint16, len(data)/2)
	for index := range units {
		units[index] = uint16(data[2*index]) | uint16(data[2*index+1])<<8
	}
	return string(utf16.Decode(units))
}

// textPipeline is what makes a launcher script decode stdin and write it
// again: a PowerShell text read of the input or a redirected stream.
func textPipeline(script string) string {
	return regexp.MustCompile(`(?i)ReadToEnd|\$input|\[Console\]::In|Redirect\w*\s*=\s*\$true`).FindString(script)
}

// TestWindowsLauncherScriptKeepsTheBytes holds the encoded launcher to the
// byte transport of D13 on every system: no text pipeline of PowerShell
// reads stdin and no stream is redirected, so the child inherits the bytes
// the host wrote, and the red rows prove the check finds the pipeline a
// launcher that reads stdin as text would carry.
func TestWindowsLauncherScriptKeepsTheBytes(t *testing.T) {
	t.Parallel()
	launcher := cinLauncherScript(t, HookCommand(`C:\Kit área\golaw.exe`, "windows", "hook", "codex"))
	for _, row := range []struct {
		name, script string
		pipeline     bool
	}{
		{"near the encoded launcher", launcher, false},
		{"red a text pipeline", "$data = [Console]::In.ReadToEnd(); $data | & 'C:\\Kit\\golaw.exe' hook codex; exit $LASTEXITCODE", true},
		{"red the input enumerator", "$input | & 'C:\\Kit\\golaw.exe' hook codex", true},
		{"red a redirected stream", "$start.RedirectStandardInput = $true", true},
	} {
		if got := textPipeline(row.script) != ""; got != row.pipeline {
			t.Errorf("%s: text pipeline = %v in %q", row.name, got, row.script)
		}
	}
	if !strings.Contains(launcher, "System.Diagnostics.Process") || !strings.Contains(launcher, "UseShellExecute") {
		t.Fatalf("the launcher script = %q", launcher)
	}
}
