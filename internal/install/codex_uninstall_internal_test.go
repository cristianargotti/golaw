package install

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCodexUninstallPreservesForeignOwnership(t *testing.T) {
	t.Parallel()
	for _, row := range []codexUninstallRow{
		{"user owned", "user", false, true},
		{"project existing", "project", false, false},
		{"user shared", "user", true, false},
		{"project shared", "project", true, true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			files := codexUninstallApplied(t, row)
			codexAssertRemoved(t, row, files)
			codexAssertRepeatedRemoval(t, files)
		})
	}
}

// codexUninstallRow is one row of TestCodexUninstallPreservesForeignOwnership.
type codexUninstallRow struct {
	name, scope     string
	shared, created bool
}

// codexUninstallFiles are the files one row installs into and removes from.
type codexUninstallFiles struct {
	options                         Options
	directory, blockPath, hooksPath string
	installed                       Plan
}

func codexUninstallApplied(t *testing.T, row codexUninstallRow) codexUninstallFiles {
	t.Helper()
	options := codexOptions(t, row.scope)
	directory := options.ConfigDir
	blockPath := filepath.Join(directory, "AGENTS.md")
	if row.scope == "project" {
		directory = filepath.Join(options.Root, ".codex")
		blockPath = filepath.Join(options.Root, "AGENTS.md")
	}
	hooksPath := filepath.Join(directory, "hooks.json")
	cinWrite(t, hooksPath, []byte(cinForeignHooks))
	if !row.created {
		cinWrite(t, blockPath, []byte("# Local\nKeep this.\n"))
	}
	installed := codexInstall(t, options)
	if row.shared {
		cinSkillReceipt(t, options.StateDir, "cursor", row.scope,
			installed.Receipt.ProjectID, SkillPath(options))
	}
	shifted := codexSurroundForeign(t, cinRead(t, hooksPath))
	cinWrite(t, hooksPath, shifted)
	plan, err := UninstallCodex(installed.ReceiptPath)
	must(t, err)
	if !plan.Uninstall || !bytes.Equal(cinRead(t, hooksPath), shifted) {
		t.Fatal("removal planning changed files")
	}
	if !strings.Contains(strings.Join(plan.Notes, "\n"), "/hooks") {
		t.Fatal("moved foreign trust identity was not reported")
	}
	must(t, Apply(t.Context(), plan, io.Discard, nil))
	return codexUninstallFiles{
		options: options, directory: directory, blockPath: blockPath,
		hooksPath: hooksPath, installed: installed,
	}
}

func codexAssertRemoved(t *testing.T, row codexUninstallRow, files codexUninstallFiles) {
	t.Helper()
	want := codexDecode(t, codexSurroundForeign(t, []byte(cinForeignHooks)))
	if !reflect.DeepEqual(codexDecode(t, cinRead(t, files.hooksPath)), want) {
		t.Fatal("removal changed foreign hooks or their order")
	}
	for _, path := range []string{
		filepath.Join(files.directory, "rules", "golaw.rules"),
		files.installed.ReceiptPath,
	} {
		if _, statErr := os.Stat(path); !errors.Is(statErr, fs.ErrNotExist) {
			t.Fatalf("owned file remains: %s: %v", path, statErr)
		}
	}
	_, skillErr := os.Stat(SkillPath(files.options))
	if row.shared && skillErr != nil {
		t.Fatalf("shared skill removed: %v", skillErr)
	}
	if !row.shared && !errors.Is(skillErr, fs.ErrNotExist) {
		t.Fatalf("unshared skill remains: %v", skillErr)
	}
	if row.created {
		if _, statErr := os.Stat(files.blockPath); !errors.Is(statErr, fs.ErrNotExist) {
			t.Fatalf("created instructions remain: %v", statErr)
		}
	} else if string(cinRead(t, files.blockPath)) != "# Local\nKeep this.\n" {
		t.Fatal("an install and uninstall cycle changed the file")
	}
}

func codexAssertRepeatedRemoval(t *testing.T, files codexUninstallFiles) {
	t.Helper()
	hooksBefore := cinRead(t, files.hooksPath)
	repeated, repeatErr := UninstallCodex(files.installed.ReceiptPath)
	if !errors.Is(repeatErr, fs.ErrNotExist) {
		t.Fatalf("repeated removal error = %v", repeatErr)
	}
	if len(repeated.Changes) != 0 {
		t.Fatalf("repeated removal changes = %+v", repeated.Changes)
	}
	if !bytes.Equal(hooksBefore, cinRead(t, files.hooksPath)) {
		t.Fatal("repeated removal changed foreign hooks")
	}
	if _, statErr := os.Stat(files.installed.ReceiptPath); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("repeated removal wrote receipt: %v", statErr)
	}
}

func codexSurroundForeign(t *testing.T, data []byte) []byte {
	t.Helper()
	document := codexDecode(t, data)
	events, ok := document["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("hooks is not a mapping: %T", document["hooks"])
	}
	extra := codexDecode(t, []byte(
		`{"matcher":"Bash","hooks":[{"type":"command","command":"later foreign hook","timeout":9}]}`,
	))
	for _, event := range []string{"PreToolUse", "PostToolUse"} {
		current, ok := events[event].([]any)
		if !ok {
			t.Fatalf("%s is not a group list", event)
		}
		groups := append([]any{extra}, current...)
		events[event] = append(groups, extra)
	}
	return codexJSON(t, document)
}

func TestCodexUninstallRefusesOwnedDrift(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		edit func(*testing.T, Options, Plan)
	}{
		{"rules", func(t *testing.T, o Options, _ Plan) {
			t.Helper()
			path := filepath.Join(o.Root, ".codex", "rules", "golaw.rules")
			cinWrite(t, path, append(cinRead(t, path), []byte("# changed\n")...))
		}},
		{"skill", func(t *testing.T, o Options, _ Plan) {
			t.Helper()
			path := SkillPath(o)
			cinWrite(t, path, append(cinRead(t, path), []byte("\nchanged\n")...))
		}},
		{"block", func(t *testing.T, _ Options, plan Plan) {
			t.Helper()
			path := plan.Receipt.Blocks[0].Path
			changed := bytes.Replace(cinRead(t, path),
				[]byte("<!-- golaw:begin -->"),
				[]byte("<!-- golaw:begin -->\nchanged"), 1)
			cinWrite(t, path, changed)
		}},
		{"handler", func(t *testing.T, _ Options, plan Plan) {
			t.Helper()
			codexChangeHandler(t, plan)
		}},
		{"ambiguous handler", func(t *testing.T, _ Options, plan Plan) {
			t.Helper()
			path := plan.Receipt.HooksPath
			document := codexDecode(t, cinRead(t, path))
			events, ok := document["hooks"].(map[string]any)
			if !ok {
				t.Fatalf("hooks is not a mapping: %T", document["hooks"])
			}
			entry := plan.Receipt.Entries[0]
			groups, ok := events[entry.Event].([]any)
			if !ok {
				t.Fatalf("%s is not a group list", entry.Event)
			}
			events[entry.Event] = append(groups, groups[entry.Group])
			cinWrite(t, path, codexJSON(t, document))
		}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			options := codexOptions(t, "project")
			installed := codexInstall(t, options)
			row.edit(t, options, installed)
			receipt := cinRead(t, installed.ReceiptPath)
			hooks := cinRead(t, installed.Receipt.HooksPath)
			if _, err := UninstallCodex(installed.ReceiptPath); !errors.Is(err, ErrChanged) {
				t.Fatalf("owned drift = %v", err)
			}
			if !bytes.Equal(receipt, cinRead(t, installed.ReceiptPath)) ||
				!bytes.Equal(hooks, cinRead(t, installed.Receipt.HooksPath)) {
				t.Fatal("refused removal changed files")
			}
		})
	}
}

// TestCodexUninstallRemovesTheHooksFileItCreated installs into a home
// without a hooks file and removes it again: nothing foreign is left, so
// the file the install created goes with it (section 5.6; added by the
// review of the plan).
func TestCodexUninstallRemovesTheHooksFileItCreated(t *testing.T) {
	t.Parallel()
	options := codexOptions(t, "user")
	installed := codexInstall(t, options)
	if !installed.Receipt.HooksCreated {
		t.Fatalf("receipt = %+v", installed.Receipt)
	}
	plan, err := UninstallCodex(installed.ReceiptPath)
	must(t, err)
	must(t, Apply(t.Context(), plan, io.Discard, nil))
	if _, statErr := os.Stat(installed.Receipt.HooksPath); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("a hooks file the install created remains: %v", statErr)
	}
}

func TestCodexUninstallWithoutReceiptDoesNotGuess(t *testing.T) {
	t.Parallel()
	options := codexOptions(t, "user")
	hooks := filepath.Join(options.ConfigDir, "hooks.json")
	cinWrite(t, hooks, []byte(cinForeignHooks))
	path := filepath.Join(options.StateDir, "codex", "user", "receipt.json")
	plan, err := UninstallCodex(path)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("absent receipt = %v", err)
	}
	if len(plan.Changes) != 0 {
		t.Fatalf("absent receipt changes = %+v", plan.Changes)
	}
	if string(cinRead(t, hooks)) != cinForeignHooks {
		t.Fatal("removal guessed ownership without a receipt")
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("removal wrote an absent receipt: %v", statErr)
	}
}
