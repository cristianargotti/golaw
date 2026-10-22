package doctor

import (
	"reflect"
	"testing"
)

func TestClaudeReportIsTheAssessmentOfItsCollection(t *testing.T) {
	t.Parallel()
	input := Input{Root: t.TempDir(), ReceiptPath: "absent.json", Agent: "claude", Scope: "user"}
	store := &enrollmentStore{}
	want := Assess(Collect(t.Context(), input, store, NativeRunner{}))
	if got := Claude(t.Context(), input, store); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestAgentsWhoseDiagnosticsAreNotBuiltReportUnknown(t *testing.T) {
	t.Parallel()
	for agent, report := range map[string]func() Report{
		"codex":  func() Report { return Codex(t.Context(), Input{Agent: "codex"}, &enrollmentStore{}) },
		"cursor": func() Report { return Cursor(t.Context(), Input{Agent: "cursor"}, &enrollmentStore{}) },
	} {
		got := report()
		want := Line{ID: "doctor.agent", Key: agent, State: stateUnknown, Code: "agent-doctor-not-built", Detail: "the " + agent + " diagnostics are not built yet"}
		if got.Schema != 1 || len(got.Lines) != 1 || got.Lines[0] != want {
			t.Errorf("%s: %+v", agent, got)
		}
	}
}
