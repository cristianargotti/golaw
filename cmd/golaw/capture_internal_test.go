package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/agents"
	"github.com/cristianargotti/golaw/internal/install"
)

func TestHookCaptureWritesTheRecordDoctorReads(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	h := hookBindingFor(t, b)
	root := repository(t, true)
	c := hook.Capture{Event: "preToolUse", Tool: "Write", Keys: []string{"content", "file_path"}}
	if err := h.capture(t.Context(), hook.Repository{Root: root}, c); err != nil {
		t.Fatal(err)
	}
	_, identity, err := install.ProjectIdentity(root)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(agents.CapturePath(h.state, h.agent, identity))
	if err != nil {
		t.Fatal(err)
	}
	var record doctor.CaptureRecord
	if err := json.Unmarshal(data, &record); err != nil || record.Identity != identity || record.HookDigest != "definition" || len(record.Captures) != 1 {
		t.Fatalf("%+v, %v", record, err)
	}
	if err := h.capture(t.Context(), hook.Repository{Root: filepath.Join(root, "absent")}, c); err == nil {
		t.Fatal("a capture of an absent root was recorded")
	}
}

func TestHookDependenciesBindTheCapture(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	deps, err := b.hookDependencies(agentUnderTest(), now())
	if err != nil {
		t.Fatal(err)
	}
	if deps.Capture == nil {
		t.Fatal("the hook dependencies record no capture")
	}
}
