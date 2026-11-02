package model_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestTheSessionShapeCarriesNoPayload(t *testing.T) {
	t.Parallel()
	if model.ShapeSession.String() != "session" || !model.ShapeSession.Valid() || model.ShapeSession != model.ShapePost+1 {
		t.Fatalf("ShapeSession = %d %q", model.ShapeSession, model.ShapeSession.String())
	}
	var shape model.EventShape
	if err := shape.UnmarshalText([]byte("session")); err != nil || shape != model.ShapeSession {
		t.Fatalf("UnmarshalText = %v, %v", shape, err)
	}
	event := model.Event{Shape: model.ShapeSession, Agent: "tester"}
	if err := event.Validate(); err != nil {
		t.Fatalf("Validate = %v", err)
	}
	event.Post = &model.PostResult{}
	if err := event.Validate(); err == nil {
		t.Fatal("a session event with a post payload validated")
	}
}
