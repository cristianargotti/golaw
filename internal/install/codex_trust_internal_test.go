package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexTrustHashGoldens(t *testing.T) {
	t.Parallel()
	cinHome(t)
	for _, row := range []struct {
		name, event, suffix, want string
	}{
		{
			"pre", "PreToolUse", `,"timeout":10`,
			"sha256:8a558a899c13b8c1879f497999eb9d26aa178f642737d2ed40bdae925cb441e3",
		},
		{
			"post", "PostToolUse", `,"timeout":10`,
			"sha256:99876b84ba43dcdba592696855b897d8b692808bccb60ae8fcaee8bee3ff2a5d",
		},
		{
			"changed timeout", "PreToolUse", `,"timeout":11`,
			"sha256:2c6612e776044a7f787db9ba831f9acdcbdb2481296211a79598244a5b419a12",
		},
		{
			"default timeout", "PreToolUse", "",
			"sha256:7c288cb997b06bf593008889602b865903e7b4a0452651da4a4b6de348dd6b19",
		},
		{
			"explicit default", "PreToolUse", `,"timeout":600`,
			"sha256:7c288cb997b06bf593008889602b865903e7b4a0452651da4a4b6de348dd6b19",
		},
		{
			"normalized defaults", "PreToolUse",
			`,"timeout":10,"async":false,"additionalContextLimit":2500`,
			"sha256:8a558a899c13b8c1879f497999eb9d26aa178f642737d2ed40bdae925cb441e3",
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			handler := `{"type":"command","command":"'/usr/local/bin/golaw' hook codex"` +
				row.suffix + `}`
			got, err := CodexTrustHash(row.event, "^(Bash|apply_patch)$",
				json.RawMessage(handler), "linux")
			if err != nil || got != row.want {
				t.Fatalf("hash = %q, %v; want %q", got, err, row.want)
			}
		})
	}
}

func TestCodexTrustHashNormalization(t *testing.T) {
	t.Parallel()
	cinHome(t)
	base := json.RawMessage(`{"type":"command","command":"portable","timeout":10}`)
	windows := json.RawMessage(
		`{"type":"command","command":"portable","commandWindows":"native","timeout":10}`)
	native := json.RawMessage(`{"type":"command","command":"native","timeout":10}`)
	baseHash := cinTrustHash(t, base, "linux")
	if cinTrustHash(t, windows, "linux") != baseHash {
		t.Fatal("Windows override changed the other platform hash")
	}
	if cinTrustHash(t, windows, "windows") != cinTrustHash(t, native, "windows") {
		t.Fatal("Windows override was not normalized")
	}
	for _, field := range []string{
		`,"async":true`,
		`,"statusMessage":"ready"`,
		`,"additionalContextLimit":0`,
	} {
		changed := json.RawMessage(string(base[:len(base)-1]) + field + "}")
		if cinTrustHash(t, changed, "linux") == baseHash {
			t.Fatalf("field omitted from hash: %s", field)
		}
	}
	zero := json.RawMessage(`{"type":"command","command":"portable","timeout":0}`)
	one := json.RawMessage(`{"type":"command","command":"portable","timeout":1}`)
	if cinTrustHash(t, zero, "linux") != cinTrustHash(t, one, "linux") {
		t.Fatal("timeout minimum was not normalized")
	}
	for _, input := range []string{
		`{`, `null`, `{}`, `{"type":"prompt"}`,
		`{"type":"command","command":7}`,
		`{"type":"command","command":"x","timeout":-1}`,
	} {
		if _, err := CodexTrustHash("PreToolUse", "", json.RawMessage(input), "linux"); err == nil {
			t.Fatalf("invalid handler accepted: %s", input)
		}
	}
}

func TestCodexTrustRecordAndNotes(t *testing.T) {
	t.Parallel()
	home := cinHome(t)
	configDir := filepath.Join(home, ".codex")
	config := filepath.Join(configDir, "config.toml")
	hooks := filepath.Join(configDir, "hooks.json")
	key := CodexTrustKey(hooks, "PreToolUse", 2, 3)
	if key != hooks+":pre_tool_use:2:3" {
		t.Fatalf("key = %q", key)
	}
	if got := CodexTrustKey(hooks, "PostToolUse", 0, 0); got != hooks+":post_tool_use:0:0" {
		t.Fatalf("post key = %q", got)
	}
	got, err := CodexTrusted(config)
	if err != nil || len(got) != 0 {
		t.Fatalf("absent record = %v, %v", got, err)
	}
	definition := json.RawMessage(
		`{"type":"command","command":"'/usr/local/bin/golaw' hook codex","timeout":10}`)
	plan := Plan{Receipt: Receipt{
		HooksPath: hooks,
		Entries: []Entry{{
			Event: "PreToolUse", Group: 2, Handler: 3,
			Matcher: "^(Bash|apply_patch)$", Definition: definition,
		}},
	}}
	options := Options{ConfigDir: configDir, GOOS: "linux"}
	must(t, codexTrustNotes(&plan, options))
	if !strings.Contains(strings.Join(plan.Notes, "\n"), "/hooks") {
		t.Fatal("missing trust instruction")
	}
	hash := cinTrustHash(t, definition, "linux")
	content := []byte(fmt.Sprintf(
		"model = \"fixture\"\n[hooks.state.%q]\ntrusted_hash = %q\nenabled = false\n",
		key, hash))
	cinWrite(t, config, content)
	got, err = CodexTrusted(config)
	if err != nil || len(got) != 1 || got[key] != hash {
		t.Fatalf("record = %v, %v", got, err)
	}
	plan.Notes = nil
	must(t, codexTrustNotes(&plan, options))
	if len(plan.Notes) != 0 {
		t.Fatalf("unchanged hash requires validation: %v", plan.Notes)
	}
	if !bytes.Equal(cinRead(t, config), content) {
		t.Fatal("trust reader changed configuration")
	}
	for _, malformed := range []string{
		"[hooks.state",
		fmt.Sprintf("[hooks.state.%q]\ntrusted_hash = 7\n", key),
	} {
		cinWrite(t, config, []byte(malformed))
		if _, readErr := CodexTrusted(config); readErr == nil {
			t.Fatalf("malformed record accepted: %q", malformed)
		}
	}
}

func cinTrustHash(t *testing.T, handler json.RawMessage, goos string) string {
	t.Helper()
	hash, err := CodexTrustHash("PreToolUse", "^(Bash|apply_patch)$", handler, goos)
	must(t, err)
	return hash
}

// TestCodexTrustHashWritesWhatCodexHashes pins, with hashes computed apart
// from the JSON Codex writes, a context limit other than the default kept,
// an absent matcher left out, the line and paragraph separators written
// raw while every other escape stays as it is, and the events golaw never
// installs refused instead of guessed.
func TestCodexTrustHashWritesWhatCodexHashes(t *testing.T) {
	t.Parallel()
	separators := "a" + string(rune(0x2028)) + "b" + string(rune(0x2029)) + "c"
	escaped := "c" + string(rune(92)) + "u2028<&>"
	for _, row := range []struct {
		name, event, matcher string
		handler              map[string]any
		want                 string
	}{
		{"post limit", "PostToolUse", "^x$", map[string]any{"additionalContextLimit": 0, "async": true}, "sha256:4bde5e848df83adfbeff45570f5989b4eb23dbf3f188422750283ca20f73cfd9"},
		{"pre no matcher", "PreToolUse", "", map[string]any{"timeout": 0}, "sha256:cd09f1ddab96c4350f1383447729106ff53cc48363a2b49e5b9eece83d37a9fb"},
		{"separators", "PreToolUse", "^x$", map[string]any{"statusMessage": separators}, "sha256:9f7e1ed766ab09100a9035f1041f4483ab4690944cfe4d5e23810fd82e1048df"},
		{"escaped text", "PreToolUse", "^x$", map[string]any{"command": escaped}, "sha256:e6df522717f2710674caefc3e23ac9dc96b80d9d8f37245b7d66059b955606b6"},
	} {
		handler := map[string]any{"type": "command", "command": "c"}
		maps.Copy(handler, row.handler)
		data, err := json.Marshal(handler)
		must(t, err)
		got, err := CodexTrustHash(row.event, row.matcher, data, "linux")
		if err != nil || got != row.want {
			t.Errorf("%s: hash = %q, %v; want %q", row.name, got, err, row.want)
		}
	}
	for _, event := range []string{"SessionEnd", "UserPromptSubmit", "Stop", "PostToolUseFailure", ""} {
		if _, err := CodexTrustHash(event, "", json.RawMessage(`{"type":"command","command":"c"}`), "linux"); !errors.Is(err, ErrSettings) {
			t.Errorf("event %q = %v", event, err)
		}
	}
}

// TestCodexTrustHashRefusesWhatCodexWouldNotRun holds the reader to the
// types Codex gives a handler and to the alias of the Windows command.
func TestCodexTrustHashRefusesWhatCodexWouldNotRun(t *testing.T) {
	t.Parallel()
	alias := json.RawMessage(`{"type":"command","command":"portable","command_windows":"native","timeout":10}`)
	native := json.RawMessage(`{"type":"command","command":"native","timeout":10}`)
	if cinTrustHash(t, alias, "windows") != cinTrustHash(t, native, "windows") {
		t.Fatal("the snake case Windows command was not read")
	}
	for _, input := range []string{
		`{"type":"command","command":"  "}`,
		`{"type":"command","command":"x","commandWindows":""}`,
		`{"type":"command","command":"x","async":null}`,
		`{"type":"command","command":"x","timeout":9223372036854775808}`,
		`{"type":"command","command":"x","timeout":1.5}`,
		`{"type":"command","command":null}`,
		`{"type":null,"command":"x"}`,
	} {
		if _, err := CodexTrustHash("PreToolUse", "", json.RawMessage(input), "windows"); !errors.Is(err, ErrSettings) {
			t.Errorf("handler %s = %v", input, err)
		}
	}
}

// TestCodexTrustRecordReadsWhatCodexReads reads a configuration reached
// through a link, as Codex reads it, keeps a state without a hash out of
// the record, and refuses what the notes cannot judge.
func TestCodexTrustRecordReadsWhatCodexReads(t *testing.T) {
	t.Parallel()
	home := cinHome(t)
	configDir := filepath.Join(home, ".codex")
	hooks := filepath.Join(configDir, "hooks.json")
	key := CodexTrustKey(hooks, "PreToolUse", 0, 0)
	dotfiles := filepath.Join(home, "dotfiles", "config.toml")
	cinWrite(t, dotfiles, []byte(fmt.Sprintf("[hooks.state.%q]\ntrusted_hash = \"sha256:x\"\n[hooks.state.\"other\"]\nenabled = false\n", key)))
	must(t, os.MkdirAll(configDir, 0o700))
	if err := os.Symlink(dotfiles, filepath.Join(configDir, "config.toml")); err != nil {
		t.Skipf("links are not available here: %v", err)
	}
	got, err := CodexTrusted(filepath.Join(configDir, "config.toml"))
	if err != nil || len(got) != 1 || got[key] != "sha256:x" {
		t.Fatalf("record through a link = %v, %v", got, err)
	}
	if _, err := CodexTrusted(configDir); err == nil {
		t.Fatal("a directory was read as the record")
	}
	plan := Plan{Receipt: Receipt{HooksPath: hooks, Entries: []Entry{{
		Event: "PreToolUse", Definition: json.RawMessage(`{"type":"prompt"}`),
	}}}}
	if err := codexTrustNotes(&plan, Options{ConfigDir: configDir}); !errors.Is(err, ErrSettings) {
		t.Fatalf("an entry that is no command = %v", err)
	}
	if err := codexTrustNotes(&plan, Options{ConfigDir: ".codex"}); !errors.Is(err, ErrPath) {
		t.Fatalf("a relative configuration directory = %v", err)
	}
	cinWrite(t, dotfiles, []byte("[hooks.state"))
	if err := codexTrustNotes(&plan, Options{ConfigDir: configDir}); !errors.Is(err, ErrSettings) {
		t.Fatalf("a malformed record = %v", err)
	}
}
