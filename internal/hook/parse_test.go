package hook_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// counting wraps the parse of an adapter with a counter of its calls.
func counting(adapter hook.Adapter, parses *int) hook.Adapter {
	parse := adapter.Parse
	adapter.Parse = func(data []byte) (hook.Parsed, error) {
		*parses++
		return parse(data)
	}
	return adapter
}

// TestTheHookParsesEachEventOnce counts the parses the adapter is asked for
// a write, a shell pre event and its post event: each event is decoded
// once, and its request is answered from that one parse.
func TestTheHookParsesEachEventOnce(t *testing.T) {
	t.Parallel()
	parses := 0
	adapter, dependencies := harness(t, request("safe"))
	if _, got := hook.Evaluate(t.Context(), "claude", []byte("{}"), counting(adapter, &parses), dependencies); got.Outcome != hook.Allow || parses != 1 {
		t.Fatalf("write: %d parses, %+v", parses, got)
	}
	req := shellRequest()
	adapter, dependencies = harness(t, req)
	parses = 0
	event, got := hook.Evaluate(t.Context(), "claude", []byte("{}"), counting(adapter, &parses), dependencies)
	if got.Outcome != hook.Allow || parses != 1 {
		t.Fatalf("shell: %d parses, %+v", parses, got)
	}
	parses = 0
	adapter = counting(asPostEvent(&req, event.Cwd, false), &parses)
	if _, got = hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies); got.Outcome != hook.Allow || parses != 1 {
		t.Fatalf("post: %d parses, %+v", parses, got)
	}
}

// TestAnAdapterWithoutADecoderCannotJudge refuses a parse that answers no
// decoder and an adapter without a parse, so a broken adapter never reads
// as an allow.
func TestAnAdapterWithoutADecoderCannotJudge(t *testing.T) {
	t.Parallel()
	adapter, dependencies := harness(t, request("safe"))
	adapter.Parse = func([]byte) (hook.Parsed, error) {
		return hook.Parsed{Event: model.Event{Agent: "claude", Shape: model.ShapeWrite}}, nil
	}
	if _, got := hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies); !got.CannotJudge {
		t.Fatalf("%+v", got)
	}
	adapter.Parse = nil
	if reply := hook.Run(t.Context(), "claude", nil, adapter, dependencies); reply.ExitCode != 2 {
		t.Fatalf("%+v", reply)
	}
}
