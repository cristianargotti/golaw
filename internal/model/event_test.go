package model_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func writeEvent() *model.Event {
	return &model.Event{
		Shape: model.ShapeWrite, Agent: "tester", Tool: "Write",
		Candidates: []model.Candidate{{Path: "internal/a.go", Op: model.CandidateCreate, Reconstructed: true}},
	}
}

func TestEventShapeText(t *testing.T) {
	t.Parallel()
	for shape := model.ShapeWrite; shape <= model.ShapePost; shape++ {
		text, err := shape.MarshalText()
		if err != nil {
			t.Fatal(err)
		}
		var back model.EventShape
		if err := back.UnmarshalText(text); err != nil || back != shape {
			t.Errorf("shape %s does not round trip: %v", shape, err)
		}
	}
	var unknown model.EventShape
	if err := unknown.UnmarshalText([]byte("edit")); !errors.Is(err, model.ErrUsage) {
		t.Errorf("unknown shape: %v", err)
	}
	if model.EventShape(0).String() != "unknown" || model.EventShape(0).Valid() {
		t.Error("the zero shape must be unknown and invalid")
	}
}

func TestCandidateOpText(t *testing.T) {
	t.Parallel()
	for op := model.CandidateCreate; op <= model.CandidateMove; op++ {
		text, err := op.MarshalText()
		if err != nil {
			t.Fatal(err)
		}
		var back model.CandidateOp
		if err := back.UnmarshalText(text); err != nil || back != op {
			t.Errorf("op %s does not round trip: %v", op, err)
		}
	}
	var unknown model.CandidateOp
	if err := unknown.UnmarshalText([]byte("append")); !errors.Is(err, model.ErrUsage) {
		t.Errorf("unknown op: %v", err)
	}
	if model.CandidateOp(0).String() != "unknown" || model.CandidateOp(0).Valid() {
		t.Error("the zero op must be unknown and invalid")
	}
}

func TestEventValidateAcceptsEveryShape(t *testing.T) {
	t.Parallel()
	events := []*model.Event{
		writeEvent(),
		{Shape: model.ShapeRead, Agent: "tester", Tool: "Read", Read: &model.ReadRequest{Path: "docs/notes.md"}},
		{Shape: model.ShapeShell, Agent: "tester", Tool: "Bash", Shell: &model.ShellCommand{Command: "git status", Cwd: "/work"}},
		{Shape: model.ShapePost, Agent: "tester", Tool: "Bash", Post: &model.PostResult{Response: "ok", Changed: []string{"a.go"}}},
		{Shape: model.ShapeWrite, Agent: "tester", Tool: "apply_patch", Candidates: []model.Candidate{{Path: "b.go", Op: model.CandidateMove, From: "a.go"}}},
	}
	for _, e := range events {
		if err := e.Validate(); err != nil {
			t.Errorf("%s event refused: %v", e.Shape, err)
		}
	}
}

func TestEventValidateRefusesMismatches(t *testing.T) {
	t.Parallel()
	cases := map[string]func(e *model.Event){
		"zero shape":         func(e *model.Event) { e.Shape = 0 },
		"bad agent id":       func(e *model.Event) { e.Agent = "Tester Two" },
		"write without":      func(e *model.Event) { e.Candidates = nil },
		"write plus read":    func(e *model.Event) { e.Read = &model.ReadRequest{Path: "x"} },
		"write plus shell":   func(e *model.Event) { e.Shell = &model.ShellCommand{Command: "ls"} },
		"write plus post":    func(e *model.Event) { e.Post = &model.PostResult{} },
		"candidate no path":  func(e *model.Event) { e.Candidates[0].Path = "" },
		"candidate zero op":  func(e *model.Event) { e.Candidates[0].Op = 0 },
		"move without from":  func(e *model.Event) { e.Candidates[0].Op = model.CandidateMove },
		"from without move":  func(e *model.Event) { e.Candidates[0].From = "old.go" },
		"read shape no read": func(e *model.Event) { e.Shape = model.ShapeRead },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := writeEvent()
			mutate(e)
			if err := e.Validate(); !errors.Is(err, model.ErrUsage) {
				t.Fatalf("Validate() = %v, want ErrUsage", err)
			}
		})
	}
	var nilEvent *model.Event
	if err := nilEvent.Validate(); !errors.Is(err, model.ErrUsage) {
		t.Errorf("nil event: %v", err)
	}
}

func TestEventJSONRoundTrip(t *testing.T) {
	t.Parallel()
	text := `{"shape":"write","agent":"tester","tool":"Write","tool_use_id":"t1",` +
		`"candidates":[{"path":"internal/a.go","op":"create","reconstructed":true}]}`
	var e model.Event
	if err := json.Unmarshal([]byte(text), &e); err != nil {
		t.Fatal(err)
	}
	if err := e.Validate(); err != nil {
		t.Fatalf("decoded event refused: %v", err)
	}
	if e.ToolUseID != "t1" || e.Candidates[0].Op != model.CandidateCreate || e.Agent != "tester" {
		t.Errorf("decoded event = %+v", e)
	}
	out, err := json.Marshal(&e)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != text {
		t.Errorf("encoded event = %s, want %s", out, text)
	}
}
