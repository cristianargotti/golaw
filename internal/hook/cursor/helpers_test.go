package cursor_test

import (
	"encoding/json"
	"maps"
	"os"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/cursor"
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
