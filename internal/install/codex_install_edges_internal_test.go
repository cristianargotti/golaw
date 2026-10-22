package install

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
)

// TestCodexHooksFileShapesCodexReads holds the reader of the hooks file to
// what Codex itself loads: a file Codex would drop whole is refused before
// anything is written, and a file it reads is merged.
func TestCodexHooksFileShapesCodexReads(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, data string
		want       error
	}{
		{"whitespace only", "  \n", nil},
		{"null description", `{"description":null,"hooks":{}}`, nil},
		{"description no string", `{"description":7,"hooks":{}}`, ErrSettings},
		{"repeated key", `{"hooks":{},"hooks":{}}`, ErrSettings},
		{"trailing document", `{"hooks":{}} {}`, ErrSettings},
		{"null event", `{"hooks":{"PreToolUse":null}}`, ErrSettings},
		{"matcher no string", `{"hooks":{"PreToolUse":[{"matcher":1,"hooks":[]}]}}`, ErrSettings},
		{"handlers no list", `{"hooks":{"PreToolUse":[{"hooks":{}}]}}`, ErrSettings},
		{"null group", `{"hooks":{"PreToolUse":[null]}}`, ErrSettings},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			options := codexOptions(t, "user")
			path := filepath.Join(options.ConfigDir, "hooks.json")
			cinWrite(t, path, []byte(row.data))
			plan, err := PlanCodex(options)
			if !errors.Is(err, row.want) {
				t.Fatalf("hooks %q = %v, want %v", row.data, err, row.want)
			}
			if row.want == nil && (plan.Receipt.HooksCreated || len(plan.Receipt.Entries) != 2) {
				t.Fatalf("a hooks file that existed = %+v", plan.Receipt)
			}
		})
	}
}

// TestCodexInstallKeepsTheBytesOfForeignValues writes the person's keys
// back in their order and their values as written: a number keeps its
// spelling and a command its characters.
func TestCodexInstallKeepsTheBytesOfForeignValues(t *testing.T) {
	t.Parallel()
	options := codexOptions(t, "user")
	path := filepath.Join(options.ConfigDir, "hooks.json")
	cinWrite(t, path, []byte(`{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"a && b","timeout":7.0}]}]},"description":"x"}`))
	codexInstall(t, options)
	text := string(cinRead(t, path))
	for _, want := range []string{`"command": "a && b"`, `"timeout": 7.0`} {
		if !strings.Contains(text, want) {
			t.Fatalf("hooks = %s, want %s", text, want)
		}
	}
	if strings.Index(text, `"hooks"`) > strings.Index(text, `"description"`) ||
		strings.Index(text, `"Stop"`) > strings.Index(text, `"PreToolUse"`) {
		t.Fatalf("key order changed: %s", text)
	}
}

// TestCodexAssetRefusesSofterDefinitions renders only the definition the
// hook's deadline and matcher were proven for.
func TestCodexAssetRefusesSofterDefinitions(t *testing.T) {
	t.Parallel()
	group := func(matcher, handler string) string {
		return `[{"matcher":"` + matcher + `","hooks":[` + handler + `]}]`
	}
	good := `{"type":"command","command":"golaw hook codex","timeout":10}`
	both := func(pre string) string {
		return `{"hooks":{"PreToolUse":` + pre + `,"PostToolUse":` + group(`^(Bash|apply_patch)$`, good) + `}}`
	}
	for name, asset := range map[string]string{
		"no JSON":         `{"hooks":`,
		"unknown field":   `{"hooks":{},"extra":1}`,
		"one event":       `{"hooks":{"PreToolUse":` + group(`^(Bash|apply_patch)$`, good) + `}}`,
		"wide matcher":    both(group("Bash", good)),
		"two handlers":    both(group(`^(Bash|apply_patch)$`, good+","+good)),
		"longer timeout":  both(group(`^(Bash|apply_patch)$`, `{"type":"command","command":"golaw hook codex","timeout":600}`)),
		"windows preset":  both(group(`^(Bash|apply_patch)$`, `{"type":"command","command":"x","commandWindows":"y","timeout":10}`)),
		"no command type": both(group(`^(Bash|apply_patch)$`, `{"type":"prompt","command":"x","timeout":10}`)),
	} {
		if _, err := renderCodexHandlers([]byte(asset), "c", ""); !errors.Is(err, ErrHooks) {
			t.Errorf("%s: %v", name, err)
		}
	}
	asset, err := assets.Read(codexHooksAsset)
	must(t, err)
	owned, err := renderCodexHandlers(asset, "c", "w")
	if err != nil || len(owned) != 2 || owned[0].event != "PreToolUse" || owned[1].event != "PostToolUse" {
		t.Fatalf("rendered = %+v, %v", owned, err)
	}
}

// TestCodexReinstallKeepsWhatItDoesNotOwn replaces golaw's handler where
// it stands and keeps a field and a handler another tool added to its group.
func TestCodexReinstallKeepsWhatItDoesNotOwn(t *testing.T) {
	t.Parallel()
	options := codexOptions(t, "user")
	installed := codexInstall(t, options)
	document := codexDecode(t, cinRead(t, installed.Receipt.HooksPath))
	group := codexFirstGroup(t, document, "PreToolUse")
	handlers, ok := group["hooks"].([]any)
	if !ok {
		t.Fatalf("handlers = %T", group["hooks"])
	}
	foreign := map[string]any{"type": "command", "command": "other", "timeout": float64(3)}
	group["note"], group["hooks"] = "keep", append([]any{foreign}, handlers...)
	cinWrite(t, installed.Receipt.HooksPath, codexJSON(t, document))
	options.Executable += "-next"
	upgraded := codexInstall(t, options)
	kept := codexFirstGroup(t, codexDecode(t, cinRead(t, installed.Receipt.HooksPath)), "PreToolUse")
	after, ok := kept["hooks"].([]any)
	if !ok || kept["note"] != "keep" || len(after) != 2 || !reflect.DeepEqual(after[0], foreign) {
		t.Fatalf("group = %#v", kept)
	}
	if upgraded.Receipt.Entries[0].Handler != 1 || !reflect.DeepEqual(after[1], codexDecode(t, upgraded.Receipt.Entries[0].Definition)) {
		t.Fatalf("entry = %+v", upgraded.Receipt.Entries[0])
	}
	if !strings.Contains(strings.Join(upgraded.Notes, "\n"), "mudou para") {
		t.Fatalf("notes = %q", upgraded.Notes)
	}
}

// codexFirstGroup is the first matcher group of event in a decoded
// hooks file.
func codexFirstGroup(t *testing.T, document map[string]any, event string) map[string]any {
	t.Helper()
	events, ok := document["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("hooks = %T", document["hooks"])
	}
	groups, ok := events[event].([]any)
	if !ok || len(groups) == 0 {
		t.Fatalf("%s = %#v", event, events[event])
	}
	group, ok := groups[0].(map[string]any)
	if !ok {
		t.Fatalf("group = %T", groups[0])
	}
	return group
}

// TestCodexReinstallAfterTheFilesChanged covers what a person may do
// between two installs: a receipt that is not Codex's is refused, a hooks
// file deleted or a block removed is written again, an edited block is
// refused, and a receipt that names another hooks file asks for the
// uninstall there first.
func TestCodexReinstallAfterTheFilesChanged(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		edit func(*testing.T, Plan)
		want error
	}{
		{"receipt of another agent", func(t *testing.T, plan Plan) {
			t.Helper()
			receipt := plan.Receipt
			receipt.Agent = "cursor"
			cinWrite(t, plan.ReceiptPath, codexJSON(t, receipt))
		}, ErrSettings},
		{"receipt of another hooks file", func(t *testing.T, plan Plan) {
			t.Helper()
			receipt := plan.Receipt
			receipt.HooksPath += ".old"
			cinWrite(t, receipt.HooksPath, cinRead(t, plan.Receipt.HooksPath))
			cinWrite(t, plan.ReceiptPath, codexJSON(t, receipt))
		}, ErrChanged},
		{"receipt of a hooks file since deleted", func(t *testing.T, plan Plan) {
			t.Helper()
			receipt := plan.Receipt
			receipt.HooksPath += ".old"
			cinWrite(t, plan.ReceiptPath, codexJSON(t, receipt))
			must(t, os.Remove(plan.Receipt.HooksPath))
		}, nil},
		{"matcher edited", func(t *testing.T, plan Plan) {
			t.Helper()
			document := codexDecode(t, cinRead(t, plan.Receipt.HooksPath))
			codexFirstGroup(t, document, "PreToolUse")["matcher"] = "Bash"
			cinWrite(t, plan.Receipt.HooksPath, codexJSON(t, document))
		}, ErrChanged},
		{"handler removed", func(t *testing.T, plan Plan) {
			t.Helper()
			document := codexDecode(t, cinRead(t, plan.Receipt.HooksPath))
			events, ok := document["hooks"].(map[string]any)
			if !ok {
				t.Fatalf("hooks = %T", document["hooks"])
			}
			delete(events, "PreToolUse")
			cinWrite(t, plan.Receipt.HooksPath, codexJSON(t, document))
		}, nil},
		{"hooks file deleted", func(t *testing.T, plan Plan) {
			t.Helper()
			must(t, os.Remove(plan.Receipt.HooksPath))
		}, nil},
		{"block edited", func(t *testing.T, plan Plan) {
			t.Helper()
			path := plan.Receipt.Blocks[0].Path
			cinWrite(t, path, []byte(strings.Replace(string(cinRead(t, path)), "golaw:begin -->", "golaw:begin -->\nmine", 1)))
		}, ErrChanged},
		{"block removed", func(t *testing.T, plan Plan) {
			t.Helper()
			cinWrite(t, plan.Receipt.Blocks[0].Path, []byte("# Mine\n"))
		}, nil},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			options := codexOptions(t, "user")
			installed := codexInstall(t, options)
			row.edit(t, installed)
			plan, err := PlanCodex(options)
			if !errors.Is(err, row.want) {
				t.Fatalf("reinstall = %v, want %v", err, row.want)
			}
			if row.want == nil && (!plan.Receipt.HooksCreated || !plan.Receipt.Blocks[0].Created || len(plan.Changes) == 0) {
				t.Fatalf("reinstall receipt = %+v, %d changes", plan.Receipt, len(plan.Changes))
			}
		})
	}
}

// TestCodexHomeThatCannotBeResolvedIsRefused names the home through a
// link loop, which Codex could not resolve either.
func TestCodexHomeThatCannotBeResolvedIsRefused(t *testing.T) {
	t.Parallel()
	options := codexOptions(t, "user")
	loop := filepath.Join(options.Home, "loop")
	if err := os.Symlink(loop, loop); err != nil {
		t.Skipf("no symbolic link can be made here: %v", err)
	}
	options.ConfigDir, options.ConfigDirSet = loop, true
	if _, err := PlanCodex(options); err == nil || errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a home through a link loop = %v", err)
	}
}
