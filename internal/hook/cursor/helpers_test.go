package cursor_test

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/cursor"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/proof"
)

func curAdapter() hook.Adapter {
	return hook.Adapter{
		Parse: cursor.Parse, Encode: cursor.Encode, Oversized: cursor.Oversized,
	}
}

func curBytes(tb testing.TB, path string) []byte {
	tb.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		tb.Fatal(err)
	}
	return data
}

func curPatch(tb testing.TB, data []byte, fields map[string]any) []byte {
	tb.Helper()
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		tb.Fatal(err)
	}
	maps.Copy(document, fields)
	encoded, err := json.Marshal(document)
	if err != nil {
		tb.Fatal(err)
	}
	return encoded
}

func curDecode(tb testing.TB, data []byte) (hook.Request, error) {
	tb.Helper()
	root := hooktest.Repository(tb)
	parsed, err := cursor.Parse(hooktest.Payload(data, root))
	if err != nil {
		return hook.Request{}, fmt.Errorf("parse: %w", err)
	}
	request, err := parsed.Decode(hook.Base{Root: root}, proof.NewTree(nil))
	if err != nil {
		return hook.Request{}, fmt.Errorf("decode: %w", err)
	}
	return request, nil
}
