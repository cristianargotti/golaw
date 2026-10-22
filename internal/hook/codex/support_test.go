package codex_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/codex"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func adapter() hook.Adapter {
	return hook.Adapter{
		Parse: codex.Parse, Encode: codex.Encode, Oversized: codex.Oversized,
	}
}

func corpus(tb testing.TB, name string) []byte {
	tb.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "payloads", name))
	if err != nil {
		tb.Fatal(err)
	}
	return data
}

func rewrite(tb testing.TB, data []byte, edit func(map[string]any)) []byte {
	tb.Helper()
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		tb.Fatal(err)
	}
	edit(value)
	out, err := json.Marshal(value)
	if err != nil {
		tb.Fatal(err)
	}
	return out
}

func commandPayload(tb testing.TB, seed, command string) []byte {
	tb.Helper()
	return rewrite(tb, corpus(tb, seed), func(value map[string]any) {
		value["tool_input"] = map[string]any{"command": command}
	})
}

func decodeRequest(tb testing.TB, data []byte, files map[string][]byte) hook.Request {
	tb.Helper()
	root := hooktest.Repository(tb)
	parsed, err := codex.Parse(hooktest.Payload(data, root))
	if err != nil {
		tb.Fatal(err)
	}
	request, err := parsed.Decode(hook.Base{Root: root}, proof.NewTree(files))
	if err != nil {
		tb.Fatal(err)
	}
	return request
}

func routeResult(tb testing.TB, directory string) (model.Event, hook.Result) {
	tb.Helper()
	fixture, err := proof.LoadFixture(filepath.Join("proof", "testdata", directory))
	if err != nil {
		tb.Fatal(err)
	}
	payload := fixture.Files[hooktest.PayloadFile]
	pre := fixture.Files[hooktest.PreFile]
	before, after := map[string][]byte{}, map[string][]byte{}
	for path, data := range fixture.Files {
		switch {
		case path == hooktest.PayloadFile || path == hooktest.PreFile:
		case strings.HasPrefix(path, hooktest.BeforeDir):
			before[strings.TrimPrefix(path, hooktest.BeforeDir)] = data
		default:
			after[path] = data
		}
	}
	root := hooktest.Repository(tb)
	dependencies := hooktest.Dependencies(tb, root, after)
	if pre != nil {
		first := hooktest.Dependencies(tb, root, before)
		first.State = dependencies.State
		_, result := hook.Evaluate(tb.Context(), "codex",
			hooktest.Payload(pre, root), adapter(), first)
		if result.Outcome != hook.Allow {
			tb.Fatalf("baseline event refused: %+v", result)
		}
	}
	return hook.Evaluate(tb.Context(), "codex",
		hooktest.Payload(payload, root), adapter(), dependencies)
}
