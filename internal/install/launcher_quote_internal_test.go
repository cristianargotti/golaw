package install

import (
	"encoding/base64"
	"encoding/binary"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"
)

// launcherEncode encodes any script text the way the launcher travels, so
// a test can hand the decoder a script HookCommand would never write.
func launcherEncode(script string) string {
	units := utf16.Encode([]rune(script))
	data := make([]byte, 2*len(units))
	for index, unit := range units {
		binary.LittleEndian.PutUint16(data[2*index:], unit)
	}
	return base64.StdEncoding.EncodeToString(data)
}

func TestCRTQuoteWritesWhatTheRuntimeReadsBack(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ arg, want string }{
		{"", `""`},
		{"hook", "hook"},
		{`a\b`, `a\b`},
		{`end\\`, `end\\`},
		{"two words", `"two words"`},
		{"tab\there", "\"tab\there\""},
		{`say "hi"`, `"say \"hi\""`},
		{`C:\Kit space\`, `"C:\Kit space\\"`},
		{`a\"b`, `"a\\\"b"`},
	} {
		if got := crtQuote(row.arg); got != row.want {
			t.Errorf("crtQuote(%q) = %q, want %q", row.arg, got, row.want)
		}
	}
}

func TestCRTWordsSplitsAsTheRuntimeDoes(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		line string
		want []string
	}{
		{"", nil},
		{"a  b", []string{"a", "b"}},
		{"a\tb", []string{"a", "b"}},
		{`""`, []string{""}},
		{`a\b`, []string{`a\b`}},
		{`trail\`, []string{`trail\`}},
		{`\\\"`, []string{`\"`}},
		{`x\\\\"y z"`, []string{`x\\y z`}},
		{`"a b" c`, []string{"a b", "c"}},
	} {
		got, ok := crtWords(row.line)
		if !ok || !slices.Equal(got, row.want) {
			t.Errorf("crtWords(%q) = %q, %v, want %q", row.line, got, ok, row.want)
		}
	}
	if words, ok := crtWords(`"open`); ok {
		t.Errorf("an open quote was split into %q", words)
	}
}

func TestLauncherArgumentsRoundTrip(t *testing.T) {
	t.Parallel()
	executable := `C:\Kit space\golaw.exe`
	for _, args := range [][]string{
		nil,
		{"hook", "codex"},
		{""},
		{"two words", `say "hi"`, `C:\dir space\`, `a\"b`, "tab\there", `end\\`},
	} {
		command := HookCommand(executable, goosWindows, args...)
		gotExecutable, gotArgs, ok := decodeLauncher(strings.TrimPrefix(command, launcherPrefix))
		if !ok || gotExecutable != executable || !slices.Equal(gotArgs, args) {
			t.Errorf("decoded %q as %q %q, %v", args, gotExecutable, gotArgs, ok)
		}
		if named, found := CommandBinary(command); !found || named != executable {
			t.Errorf("CommandBinary(%q) = %q, %v", command, named, found)
		}
	}
}

func TestLauncherRejectsAlteredScripts(t *testing.T) {
	t.Parallel()
	valid := launcherScript(`C:\golaw.exe`, "hook codex")
	for _, row := range []struct{ name, script string }{
		{"other script", "Write-Output 1\n"},
		{"bare executable", scriptOpen + `C:\golaw.exe` + scriptArgs + "'hook codex'" + scriptClose},
		{"lone apostrophe", scriptOpen + "'" + scriptArgs + "'hook codex'" + scriptClose},
		{"no arguments line", scriptOpen + `'C:\golaw.exe'` + scriptClose},
		{"no closing lines", scriptOpen + `'C:\golaw.exe'` + scriptArgs + "'hook codex'"},
		{"trailing statement", valid + "Write-Output 1\n"},
		{"open quote", launcherScript(`C:\golaw.exe`, `"open`)},
	} {
		encoded := launcherEncode(row.script)
		if executable, args, ok := decodeLauncher(encoded); ok {
			t.Errorf("%s: decoded %q %q", row.name, executable, args)
		}
		if named, ok := CommandBinary(launcherPrefix + encoded); ok {
			t.Errorf("%s: accepted as %q", row.name, named)
		}
	}
	if _, _, ok := decodeLauncher(launcherEncode(valid)); !ok {
		t.Error("the valid script was refused")
	}
}
