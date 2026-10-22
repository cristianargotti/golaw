package install

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCodexInstallRefusesInvalidHooks(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, data string
	}{
		{"truncated", `{"hooks":`},
		{"foreign top level", `{"hooks":{},"unrecognized":"keep"}`},
		{"wrong hooks type", `{"hooks":[]}`},
		{"wrong group type", `{"hooks":{"PreToolUse":["keep"]}}`},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			options := codexOptions(t, "user")
			path := filepath.Join(options.ConfigDir, "hooks.json")
			cinWrite(t, path, []byte(row.data))
			if _, err := PlanCodex(options); !errors.Is(err, ErrSettings) {
				t.Fatalf("invalid hooks = %v", err)
			}
			if string(cinRead(t, path)) != row.data {
				t.Fatal("invalid hooks file changed")
			}
		})
	}
}

func TestCodexInstallPathAndOwnershipBoundaries(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, scope string
		edit        func(*testing.T, *Options)
		want        error
	}{
		{"unknown scope", "user", func(_ *testing.T, o *Options) { o.Scope = "elsewhere" }, ErrPath},
		{"relative home", "user", func(_ *testing.T, o *Options) { o.Home = "relative" }, ErrPath},
		{"relative state", "user", func(_ *testing.T, o *Options) { o.StateDir = "relative" }, ErrPath},
		{"relative config", "user", func(_ *testing.T, o *Options) { o.ConfigDir = "relative" }, ErrPath},
		{"relative binary", "user", func(_ *testing.T, o *Options) { o.Executable = "golaw" }, ErrPath},
		{"unequal skill", "user", func(t *testing.T, o *Options) {
			t.Helper()
			cinWrite(t, SkillPath(*o), []byte("Personal skill.\n"))
		}, ErrChanged},
		{"unowned rules", "project", func(t *testing.T, o *Options) {
			t.Helper()
			cinWrite(t, filepath.Join(o.Root, ".codex", "rules", "golaw.rules"), []byte("# personal\n"))
		}, ErrChanged},
		{"nonregular rules", "project", func(t *testing.T, o *Options) {
			t.Helper()
			must(t, os.MkdirAll(filepath.Join(o.Root, ".codex", "rules", "golaw.rules"), 0o700))
		}, ErrPath},
		{"personal user rules left alone", "user", func(t *testing.T, o *Options) {
			t.Helper()
			cinWrite(t, filepath.Join(o.ConfigDir, "rules", "golaw.rules"), []byte("# personal\n"))
		}, nil},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			options := codexOptions(t, row.scope)
			row.edit(t, &options)
			if _, err := PlanCodex(options); !errors.Is(err, row.want) {
				t.Fatalf("boundary error = %v, want %v", err, row.want)
			}
		})
	}
}

func TestCodexInstallDefaultScopeAndWindowsDefinition(t *testing.T) {
	t.Parallel()
	options := codexOptions(t, "")
	options.GOOS = "windows"
	plan, err := PlanCodex(options)
	must(t, err)
	if plan.Receipt.Scope != "user" {
		t.Fatalf("default scope = %q", plan.Receipt.Scope)
	}
	for _, entry := range plan.Receipt.Entries {
		handler := codexDecode(t, entry.Definition)
		if handler["commandWindows"] != HookCommand(
			options.Executable, "windows", "hook", "codex") {
			t.Fatalf("Windows definition = %#v", handler)
		}
		if _, found := handler["failClosed"]; found {
			t.Fatal("unsupported failClosed field written")
		}
		if _, found := handler["onFailure"]; found {
			t.Fatal("unsupported onFailure field written")
		}
	}
}

func TestCodexInstallRefusesChangedOwnedHandler(t *testing.T) {
	t.Parallel()
	options := codexOptions(t, "user")
	plan := codexInstall(t, options)
	codexChangeHandler(t, plan)
	if _, err := PlanCodex(options); !errors.Is(err, ErrChanged) {
		t.Fatalf("changed handler = %v", err)
	}
}
