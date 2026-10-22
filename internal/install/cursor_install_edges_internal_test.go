package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCursorPlanRefusesInputsItCannotPlanFrom(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name  string
		edit  func(t *testing.T, options *Options)
		want  error
		scope string
	}{
		{"unknown scope", func(_ *testing.T, options *Options) { options.Scope = "global" }, ErrPath, "user"},
		{"relative home", func(_ *testing.T, options *Options) { options.Home = "home" }, ErrPath, "user"},
		{"no running binary", func(_ *testing.T, options *Options) { options.Executable = "" }, ErrPath, "user"},
		{"receipt of another agent", func(t *testing.T, options *Options) {
			t.Helper()
			path := filepath.Join(options.StateDir, "cursor", "user", "receipt.json")
			cursorWriteReceipt(t, path, Receipt{Schema: 1, ProjectID: "p", Scope: "user", Bundle: filepath.Dir(path), Agent: "codex"})
		}, ErrPath, "user"},
		{"personal project rule", func(t *testing.T, options *Options) {
			t.Helper()
			must(t, WriteAtomic(filepath.Join(options.Root, ".cursor", "rules", "golaw.mdc"), []byte("mine\n")))
		}, ErrChanged, "project"},
		{"hooks file of no version 1", func(t *testing.T, options *Options) {
			t.Helper()
			must(t, WriteAtomic(filepath.Join(options.ConfigDir, "hooks.json"), []byte(`{"version":2,"hooks":{}}`)))
		}, ErrHooks, "user"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			options := cursorOptions(t, row.scope)
			row.edit(t, &options)
			if _, err := PlanCursor(options); !errors.Is(err, row.want) {
				t.Fatalf("error = %v, want %v", err, row.want)
			}
		})
	}
}

func TestCursorPlanRefusesALinkOnTheWay(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ name, scope, link string }{
		{"project cursor folder", "project", ".cursor"},
		{"project rules folder", "project", filepath.Join(".cursor", "rules")},
		{"user cursor folder", "user", ".cursor"},
		{"user skill folder", "user", ".agents"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			options := cursorOptions(t, row.scope)
			base := options.Home
			if row.scope == "project" {
				base = options.Root
			}
			target := filepath.Join(options.Home, "elsewhere")
			must(t, os.MkdirAll(target, 0o700))
			link := filepath.Join(base, row.link)
			must(t, os.MkdirAll(filepath.Dir(link), 0o700))
			if err := os.Symlink(target, link); err != nil {
				t.Skipf("no symbolic link can be made here: %v", err)
			}
			if _, err := PlanCursor(options); !errors.Is(err, ErrPath) {
				t.Fatalf("error = %v, want %v", err, ErrPath)
			}
		})
	}
}

func TestCursorHooksFileShapesCursorReads(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, data string
		want       error
	}{
		{"malformed", `{"hooks":`, ErrHooks},
		{"hooks no object", `{"version":1,"hooks":[]}`, ErrHooks},
		{"event no list", `{"version":1,"hooks":{"preToolUse":{}}}`, ErrHooks},
		{"handler no object", `{"version":1,"hooks":{"preToolUse":[1]}}`, ErrHooks},
		{"empty file", " \n", nil},
		{"no version, another key and event kept", `{"hooks":{"stop":[{"command":"other"}]},"theme":"dark"}`, nil},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			cursorShape(t, row.data, row.want, row.name != "empty file")
		})
	}
}

// cursorShape installs over a hooks file holding data and checks the error
// and, once installed, that the version leads and what was there stays.
func cursorShape(t *testing.T, data string, want error, kept bool) {
	t.Helper()
	options := cursorOptions(t, "user")
	path := filepath.Join(options.ConfigDir, "hooks.json")
	must(t, WriteAtomic(path, []byte(data)))
	plan, err := PlanCursor(options)
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	if want != nil {
		return
	}
	cursorApply(t, plan)
	handlers := cursorHandlers(t, path)
	if len(handlers["preToolUse"]) != 1 || (kept && len(handlers["stop"]) != 1) {
		t.Fatalf("handlers = %#v", handlers)
	}
	if written := cursorRead(t, path); !bytes.HasPrefix(written, []byte("{\n  \"version\": 1,")) ||
		(kept && !bytes.Contains(written, []byte(`"theme": "dark"`))) {
		t.Fatalf("hooks file = %s", written)
	}
}

func TestCursorReinstallAfterTheHooksChanged(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		edit func(t *testing.T, path string, handlers map[string][]json.RawMessage)
		want error
	}{
		{"handler removed", func(_ *testing.T, _ string, handlers map[string][]json.RawMessage) {
			delete(handlers, "preToolUse")
		}, nil},
		{"hooks file removed", func(t *testing.T, path string, _ map[string][]json.RawMessage) {
			t.Helper()
			must(t, os.Remove(path))
		}, nil},
		{"handler edited", func(_ *testing.T, _ string, handlers map[string][]json.RawMessage) {
			handlers["preToolUse"][0] = bytes.Replace(handlers["preToolUse"][0], []byte(`"timeout":10`), []byte(`"timeout":30`), 1)
		}, ErrChanged},
		{"handler copied", func(_ *testing.T, _ string, handlers map[string][]json.RawMessage) {
			handlers["postToolUse"] = append(handlers["postToolUse"], handlers["postToolUse"][0])
		}, ErrChanged},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			options, plan := cursorInstalled(t, "user")
			path := plan.Receipt.HooksPath
			handlers := cursorCompactHandlers(t, path)
			row.edit(t, path, handlers)
			if _, statErr := os.Stat(path); statErr == nil {
				cursorRewrite(t, path, handlers)
			}
			again, err := PlanCursor(options)
			if !errors.Is(err, row.want) {
				t.Fatalf("error = %v, want %v", err, row.want)
			}
			if row.want == nil && (len(again.Receipt.Entries) != 5 || !again.Receipt.HooksCreated) {
				t.Fatalf("receipt = %+v", again.Receipt)
			}
		})
	}
}

// cursorCompactHandlers reads the handlers of a hooks file compact, the
// form a test edits by bytes.
func cursorCompactHandlers(t *testing.T, path string) map[string][]json.RawMessage {
	t.Helper()
	handlers := cursorHandlers(t, path)
	for _, list := range handlers {
		for index := range list {
			compact, err := cursorCompact(list[index])
			must(t, err)
			list[index] = compact
		}
	}
	return handlers
}

// cursorRewrite writes handlers back as another tool would, with the
// top-level keys in another order.
func cursorRewrite(t *testing.T, path string, handlers map[string][]json.RawMessage) {
	t.Helper()
	data, err := json.Marshal(map[string]any{"version": 1, "hooks": handlers})
	must(t, err)
	must(t, WriteAtomic(path, data))
}

func TestCursorReceiptOfAnotherHooksFileIsRefused(t *testing.T) {
	t.Parallel()
	options, plan := cursorInstalled(t, "user")
	options.ConfigDir = filepath.Join(options.Home, "other", ".cursor")
	if _, err := PlanCursor(options); !errors.Is(err, ErrChanged) {
		t.Fatalf("error = %v, want %v", err, ErrChanged)
	}
	if _, err := os.Stat(plan.Receipt.HooksPath); err != nil {
		t.Fatal(err)
	}
}

func TestCursorAssetRefusesSofterDefinitions(t *testing.T) {
	t.Parallel()
	handler := `{"command":"golaw hook cursor","timeout":10,"failClosed":true}`
	pre := `{"command":"golaw hook cursor","matcher":"^(Write|Delete|Shell)$","timeout":10,"failClosed":true}`
	events := func(first, rest string) string {
		return `{"version":1,"hooks":{"preToolUse":[` + first + `],"beforeReadFile":[` + rest + `],"beforeShellExecution":[` + rest +
			`],"postToolUse":[` + rest + `],"postToolUseFailure":[` + rest + `]}}`
	}
	for _, row := range []struct{ name, asset string }{
		{"malformed", `{`},
		{"unknown field", `{"version":1,"hooks":{},"extra":1}`},
		{"missing events", `{"version":1,"hooks":{"preToolUse":[` + pre + `]}}`},
		{"unanchored matcher", events(handler, handler)},
		{"longer timeout", events(pre, `{"command":"golaw hook cursor","timeout":60,"failClosed":true}`)},
		{"failing open", events(pre, `{"command":"golaw hook cursor","timeout":10}`)},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if _, err := renderCursorHandlers([]byte(row.asset), "golaw"); !errors.Is(err, ErrHooks) {
				t.Fatalf("error = %v", err)
			}
		})
	}
	if _, err := renderCursorHandlers([]byte(events(pre, handler)), "golaw"); err != nil {
		t.Fatalf("near miss: %v", err)
	}
	if _, err := cursorCompact(json.RawMessage(`{`)); err == nil || cursorMatcherOf(json.RawMessage(`[`)) != "" {
		t.Fatal("malformed handler read as one")
	}
}

// An edit that also moved golaw's handler to an index another handler
// never held is still read as golaw's, by the binary its command names.
func TestCursorEditedHandlerThatMovedIsRefused(t *testing.T) {
	t.Parallel()
	options := cursorOptions(t, "user")
	path := filepath.Join(options.ConfigDir, "hooks.json")
	must(t, WriteAtomic(path, cursorRead(t, "testdata/cursor_foreign.json")))
	plan, err := PlanCursor(options)
	must(t, err)
	cursorApply(t, plan)
	handlers := cursorCompactHandlers(t, path)
	edited := bytes.Replace(handlers["preToolUse"][2], []byte(`"timeout":10`), []byte(`"timeout":30`), 1)
	handlers["preToolUse"] = []json.RawMessage{edited}
	cursorRewrite(t, path, handlers)
	if _, err := PlanCursor(options); !errors.Is(err, ErrChanged) {
		t.Fatalf("error = %v, want %v", err, ErrChanged)
	}
}
