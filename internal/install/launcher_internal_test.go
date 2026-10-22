package install

import (
	"encoding/base64"
	"path/filepath"
	"strings"
	"testing"
)

func TestHookCommandRoundTrip(t *testing.T) {
	t.Parallel()
	home := cinHome(t)
	for _, row := range []struct {
		name, executable, goos string
	}{
		{"linux", filepath.Join(home, "bin", "golaw"), "linux"},
		{"darwin quotes", filepath.Join(home, "á space", "o'neil", "golaw"), "darwin"},
		{"windows", `C:\Kit área\o'neil $ &\golaw.exe`, "windows"},
		{"windows unc", `\\server\share\kit space\golaw.exe`, "windows"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			command := HookCommand(row.executable, row.goos, "hook", "codex")
			got, ok := CommandBinary(command)
			if !ok || got != row.executable {
				t.Fatalf("decoded = %q, %v; command = %q", got, ok, command)
			}
			if row.goos == "windows" {
				assertEncodedLauncher(t, command)
			}
		})
	}
	if got := HookCommand("/opt/kit/golaw", "linux", "hook", "codex"); got != "'/opt/kit/golaw' hook codex" {
		t.Fatalf("POSIX command = %q", got)
	}
}

// assertEncodedLauncher holds a Windows command to the outer invocation of
// D13 with an even number of script bytes behind it.
func assertEncodedLauncher(t *testing.T, command string) {
	t.Helper()
	fields := strings.Fields(command)
	if len(fields) != 9 ||
		fields[0] != "powershell.exe" ||
		fields[7] != "-EncodedCommand" {
		t.Fatalf("launcher fields = %q", fields)
	}
	data, err := base64.StdEncoding.DecodeString(fields[8])
	if err != nil || len(data) == 0 || len(data)%2 != 0 {
		t.Fatalf("encoded script = %x, %v", data, err)
	}
}

func TestCommandBinaryRejectsOtherCommands(t *testing.T) {
	t.Parallel()
	home := cinHome(t)
	executable := filepath.Join(home, "bin", "golaw")
	valid := HookCommand(executable, "windows", "hook", "codex")
	prefix := strings.TrimSuffix(valid, strings.Fields(valid)[8])
	for _, command := range []string{
		"",
		"golaw hook codex",
		"echo '/opt/kit/golaw' hook codex",
		"'/opt/kit/golaw' hook codex; other",
		valid + " ; other",
		prefix + "###",
		prefix + base64.StdEncoding.EncodeToString([]byte{1}),
		prefix + base64.StdEncoding.EncodeToString([]byte{0, 216}),
	} {
		if binary, ok := CommandBinary(command); ok {
			t.Errorf("accepted %q as %q", command, binary)
		}
	}
}

func TestCodexCommandsUseTheTargetPlatform(t *testing.T) {
	t.Parallel()
	home := cinHome(t)
	executable := filepath.Join(home, "bin", "golaw")
	for _, goos := range []string{"linux", "darwin", "windows"} {
		command, windows, err := codexCommands(Options{
			Executable: executable, GOOS: goos,
		})
		must(t, err)
		if command != HookCommand(executable, "linux", "hook", "codex") {
			t.Fatalf("portable command = %q", command)
		}
		want := ""
		if goos == "windows" {
			want = HookCommand(executable, "windows", "hook", "codex")
		}
		if windows != want {
			t.Fatalf("Windows command = %q, want %q", windows, want)
		}
	}
}
