package cursor_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/cursor"
)

func TestCursorA15DimensionsRemainDistinct(t *testing.T) {
	t.Parallel()
	data := curBytes(t, "proof/testdata/post-shell/near/payload.json")
	parsed, err := cursor.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	first, err := hook.StateKey("repository", parsed.Event)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"conversation_id", "generation_id", "tool_use_id"} {
		t.Run(field, func(t *testing.T) {
			t.Parallel()
			changed := curPatch(t, data, map[string]any{field: "another"})
			next, parseErr := cursor.Parse(changed)
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			key, keyErr := hook.StateKey("repository", next.Event)
			if keyErr != nil || key == first {
				t.Fatalf("identity collision: %q %v", key, keyErr)
			}
		})
	}
	other, err := hook.StateKey("another-repository", parsed.Event)
	if err != nil || other == first {
		t.Fatalf("repository collision: %q %v", other, err)
	}
}
