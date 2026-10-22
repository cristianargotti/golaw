package install

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

const cinForeignHooks = `{
  "description": "keep this description",
  "hooks": {
    "PreToolUse": [
      {"matcher":"Bash","hooks":[{"type":"command","command":"other pre","timeout":7,"extra":"keep"}]}
    ],
    "PostToolUse": [
      {"matcher":"Bash","hooks":[{"type":"command","command":"other post","timeout":8}]}
    ],
    "Stop": [
      {"hooks":[{"type":"command","command":"other stop"}]}
    ]
  }
}`

type cinEnrollment struct{}

func (cinEnrollment) WriteEnrollment(
	_ context.Context, _, policyDigest, profileDigest string,
) (model.Enrollment, error) {
	return model.Enrollment{
		State:        model.EnrollmentEnrolled,
		PolicyDigest: policyDigest, ProfileDigest: profileDigest,
	}, nil
}

func codexOptions(t *testing.T, scope string) Options {
	t.Helper()
	home := cinHome(t)
	options := Options{
		Home: home, Root: filepath.Join(home, "repository"),
		ConfigDir:  filepath.Join(home, ".codex"),
		StateDir:   filepath.Join(home, "state"),
		Executable: filepath.Join(home, "bin", "golaw"),
		GOOS:       "linux", Scope: scope, Version: "0.4.0",
		PolicyDigest: "policy", ProfileDigest: "profile",
	}
	must(t, os.MkdirAll(options.Root, 0o700))
	return options
}

func codexInstall(t *testing.T, options Options) Plan {
	t.Helper()
	plan, err := PlanCodex(options)
	must(t, err)
	must(t, Apply(t.Context(), plan, io.Discard, cinEnrollment{}))
	return plan
}

func codexDecode(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var value map[string]any
	must(t, json.Unmarshal(data, &value))
	return value
}

func codexJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	must(t, err)
	return append(data, '\n')
}

func codexChangeHandler(t *testing.T, plan Plan) {
	t.Helper()
	path := plan.Receipt.HooksPath
	document := codexDecode(t, cinRead(t, path))
	events, ok := document["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("hooks is not a mapping: %T", document["hooks"])
	}
	entry := plan.Receipt.Entries[0]
	groups, ok := events[entry.Event].([]any)
	if !ok {
		t.Fatalf("%s is not a group list", entry.Event)
	}
	group, ok := groups[entry.Group].(map[string]any)
	if !ok {
		t.Fatalf("%s group is not a mapping", entry.Event)
	}
	handlers, ok := group["hooks"].([]any)
	if !ok {
		t.Fatalf("%s hooks is not a handler list", entry.Event)
	}
	handler, ok := handlers[entry.Handler].(map[string]any)
	if !ok {
		t.Fatalf("%s handler is not a mapping", entry.Event)
	}
	handler["timeout"] = float64(11)
	cinWrite(t, path, codexJSON(t, document))
}
