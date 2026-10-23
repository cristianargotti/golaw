package release_test

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/cristianargotti/golaw/internal/release"
	"github.com/cristianargotti/golaw/internal/releasetest"
)

// binary is the content of the generated release's binary.
const binary = "golaw release binary\n"

// material is a generated release of 1.2.3 read as Verify reads it, and
// the material, for a test that tampers with one piece.
func material(t *testing.T) (release.Inputs, releasetest.Material) {
	t.Helper()
	m := releasetest.Generate(t, []byte(binary), "1.2.3")
	return m.Inputs(t), m
}

// edited answers a JSON document with change applied to its decoded form.
func edited(t *testing.T, data []byte, change func(doc map[string]any)) []byte {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	change(doc)
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// object is the JSON object at path under doc: a key, or the index of a
// list element.
func object(t *testing.T, doc map[string]any, path ...string) map[string]any {
	t.Helper()
	var node any = doc
	for _, key := range path {
		switch value := node.(type) {
		case map[string]any:
			node = value[key]
		case []any:
			index, err := strconv.Atoi(key)
			if err != nil || index >= len(value) {
				t.Fatalf("no element %s", key)
			}
			node = value[index]
		}
	}
	out, ok := node.(map[string]any)
	if !ok {
		t.Fatalf("no object at %v", path)
	}
	return out
}
