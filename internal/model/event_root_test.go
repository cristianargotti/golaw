package model_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestShellCommandCarriesTheRootTheHookJudgesIn(t *testing.T) {
	t.Parallel()
	data, err := json.Marshal(model.ShellCommand{Command: "ls", Cwd: "/project", Tool: "Bash", Root: "/project"})
	if err != nil || !strings.Contains(string(data), `"root":"/project"`) {
		t.Fatalf("json = %s, %v", data, err)
	}
	if data, err := json.Marshal(model.ShellCommand{Command: "ls"}); err != nil || strings.Contains(string(data), "root") {
		t.Errorf("an empty root is written: %s, %v", data, err)
	}
}
