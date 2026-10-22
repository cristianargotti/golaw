package cursor_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/cursor"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

type cursorCorpusCase struct {
	file     string
	shape    model.EventShape
	tool     string
	use      string
	observe  bool
	detect   bool
	command  string
	response string
	failed   bool
}

func TestCursorSurveyCorpus(t *testing.T) {
	t.Parallel()
	tests := []cursorCorpusCase{
		{
			file: "pre-tool-use-shell.json", shape: model.ShapeShell,
			tool: "Shell", use: "tool", command: "go test ./...",
		},
		{
			file: "pre-tool-use-write-unknown.json", shape: model.ShapeWrite,
			tool: "Write", use: "tool", observe: true,
		},
		{file: "before-read-file.json", shape: model.ShapeRead, tool: "Read"},
		{
			file: "before-shell-execution.json", shape: model.ShapeShell,
			tool: "Shell", command: "git commit -m 'feat: add hello'",
		},
		{
			file: "post-tool-use-shell.json", shape: model.ShapePost,
			tool: "Shell", use: "tool", detect: true,
			response: `{"exitCode":0,"stdout":"ok"}`,
		},
		{
			file: "post-tool-use-failure-shell.json", shape: model.ShapePost,
			tool: "Shell", use: "tool", detect: true, failed: true,
			response: "Command exited with code 1",
		},
	}
	for _, test := range tests {
		t.Run(test.file, func(t *testing.T) {
			t.Parallel()
			root := hooktest.Repository(t)
			data := hooktest.Payload(curBytes(t, "testdata/payloads/"+test.file), root)
			parsed, err := cursor.Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			request, err := parsed.Decode(hook.Base{Root: root}, proof.NewTree(nil))
			if err != nil {
				t.Fatal(err)
			}
			curCorpusIdentity(t, request, root, test)
			curCorpusPayload(t, request, test)
		})
	}
}

func curCorpusIdentity(t *testing.T, request hook.Request, root string, want cursorCorpusCase) {
	t.Helper()
	event := request.Event
	if event.Agent != "cursor" || event.Shape != want.shape || event.Tool != want.tool {
		t.Fatalf("event = %+v", event)
	}
	if event.SessionID != "conversation" || event.AgentID != "generation" ||
		event.ToolUseID != want.use || event.Cwd != root {
		t.Fatalf("identity = %+v", event)
	}
	if request.Observe != want.observe || request.Detect != want.detect {
		t.Fatalf("route = %+v", request)
	}
}

func curCorpusPayload(t *testing.T, request hook.Request, want cursorCorpusCase) {
	t.Helper()
	switch want.shape {
	case model.ShapeWrite:
		curCorpusCapture(t, request)
	case model.ShapeRead:
		curCorpusRead(t, request)
	case model.ShapeShell:
		if request.Event.Shell == nil {
			t.Fatal("shell request is absent")
		}
		if request.Event.Shell.Command != want.command ||
			request.Event.Shell.Tool != "Shell" ||
			request.Event.Shell.Cwd != request.Event.Cwd {
			t.Fatalf("shell = %+v", request.Event.Shell)
		}
	case model.ShapePost:
		if request.Event.Post == nil {
			t.Fatal("post request is absent")
		}
		if request.Event.Post.Response != want.response ||
			request.Event.Post.Failed != want.failed {
			t.Fatalf("post = %+v", request.Event.Post)
		}
	}
}

func curCorpusCapture(t *testing.T, request hook.Request) {
	t.Helper()
	want := &hook.Capture{
		Event: "preToolUse", Tool: "Write", Keys: []string{"__undocumented__"},
	}
	if !reflect.DeepEqual(request.Capture, want) {
		t.Fatalf("capture = %+v", request.Capture)
	}
	if request.Event.Target != "" || len(request.Event.Candidates) != 0 ||
		len(request.Content) != 0 {
		t.Fatalf("capture enforced a write: %+v", request)
	}
}

func curCorpusRead(t *testing.T, request hook.Request) {
	t.Helper()
	if request.Event.Read == nil || request.Event.Read.Path != ".env" ||
		len(request.Content) != 0 {
		t.Fatalf("read = %+v", request)
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "NAME=value") {
		t.Fatal("read content reached the request")
	}
}
