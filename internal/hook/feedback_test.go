package hook_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/claude"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
	detect "github.com/cristianargotti/golaw/internal/surface"
)

const digest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func claudeAdapter() hook.Adapter {
	return hook.Adapter{Parse: claude.Parse, Encode: claude.Encode, Oversized: claude.Oversized}
}

// written is the payload of a Write tool of the agent, before or after it
// ran, at a path of the repository root of the corpus.
func written(t *testing.T, event, path, content string) []byte {
	t.Helper()
	payload := map[string]any{
		"session_id": "session", "agent_id": "agent", "tool_use_id": "tool", "cwd": hooktest.Root,
		"hook_event_name": event, "tool_name": "Write",
		"tool_input": map[string]any{"file_path": hooktest.Root + "/" + path, "content": content},
	}
	if event == "PostToolUse" {
		payload["tool_response"] = map[string]any{"success": true}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// recorder plays the fast tools: it records the files it was given and
// answers report.
type recorder struct {
	mu     sync.Mutex
	files  []hook.ToolFile
	report hook.ToolReport
}

func (r *recorder) tools(_ context.Context, _ hook.Repository, _ *model.EffectivePolicy, files []hook.ToolFile) hook.ToolReport {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.files = append(r.files, files...)
	return r.report
}

// judge evaluates the payload over the files with the fast tools of rec,
// under a policy whose hook feedback is on unless off is set.
func judge(t *testing.T, payload []byte, files map[string][]byte, rec *recorder, off bool) hook.Result {
	t.Helper()
	root := hooktest.Repository(t)
	deps := hooktest.Dependencies(t, root, files)
	deps.Tools = rec.tools
	policy := deps.Policy
	deps.Policy = func(ctx context.Context, r hook.Repository) (*model.EffectivePolicy, error) {
		p, err := policy(ctx, r)
		if err != nil {
			return nil, err
		}
		changed := *p
		changed.Ladder.HookFeedback = !off
		return &changed, nil
	}
	_, result := hook.Evaluate(t.Context(), "claude", hooktest.Payload(payload, root), claudeAdapter(), deps)
	return result
}

func TestAPostWriteFeedsTheNativeNotesBack(t *testing.T) {
	t.Parallel()
	dockerfile := "FROM alpine@" + digest + "\n"
	result := judge(t, written(t, "PostToolUse", "Dockerfile", dockerfile), map[string][]byte{"Dockerfile": []byte(dockerfile)}, &recorder{}, false)
	want := "Dockerfile:1 docker.final-user (docker:S6471) the final stage sets no USER, so the container runs as root (Sonar S6471): end the final stage with USER and a numeric user other than 0, such as USER 65532:65532."
	if result.Outcome != hook.Allow || len(result.Lines) != 1 || result.Lines[0] != want {
		t.Fatalf("result = %+v\nwant one line %q", result, want)
	}
	if result.Human != "golaw enviou ao agente 1 apontamento sobre o que ele escreveu; o pipeline julga o mesmo." {
		t.Fatalf("human = %q", result.Human)
	}
}

func TestAPostWriteRunsTheFastToolsOnTheBytesWritten(t *testing.T) {
	t.Parallel()
	content := "import os\n\nvalue = os.environ[\"X\"]\n"
	rec := &recorder{report: hook.ToolReport{Findings: []model.Finding{{
		Check: "lint.ruff", Rule: "ruff/S105", Path: "test/deploy/check_install.py", Line: 3, Code: "s105",
		Message: "Possible hardcoded secret", Sonar: "python:S2068", Severity: model.SeverityFail,
	}}}}
	result := judge(t, written(t, "PostToolUse", "test/deploy/check_install.py", content), map[string][]byte{"test/deploy/check_install.py": []byte(content)}, rec, false)
	if len(rec.files) != 1 || rec.files[0].Path != "test/deploy/check_install.py" || string(rec.files[0].Content) != content || rec.files[0].Language != detect.Python {
		t.Fatalf("tool files = %+v", rec.files)
	}
	if result.Outcome != hook.Allow || len(result.Lines) != 1 || result.Lines[0] != "test/deploy/check_install.py:3 ruff/S105 (python:S2068) Possible hardcoded secret" {
		t.Fatalf("result = %+v", result)
	}
}

func TestAMissingOrKilledToolIsANoticeNeverADenial(t *testing.T) {
	t.Parallel()
	content := "#!/bin/sh\necho ok\n"
	agent := "shellcheck 0.11.0 is not installed: scripts/smoke.sh was judged by the native checks only; run golaw setup --surface, or the pipeline judges the rest."
	person := "shellcheck 0.11.0 não está instalado: o golaw julgou scripts/smoke.sh só com as verificações nativas; rode golaw setup --surface ou deixe o resto para o pipeline."
	rec := &recorder{report: hook.ToolReport{Missing: []string{agent}, Person: []string{person}, Killed: []string{"scripts/smoke.sh"}}}
	result := judge(t, written(t, "PostToolUse", "scripts/smoke.sh", content), map[string][]byte{"scripts/smoke.sh": []byte(content)}, rec, false)
	if result.Outcome != hook.Allow || result.CannotJudge || !strings.Contains(result.Notice, agent) || !strings.Contains(result.Human, person) {
		t.Fatalf("missing = %+v", result)
	}
	killed := "golaw did not measure scripts/smoke.sh: its fast linter passed the hook's time limit or answered output golaw does not read; the pipeline judges it."
	if !strings.Contains(result.Notice, killed) {
		t.Fatalf("notice = %q, want %q", result.Notice, killed)
	}
}

func TestTheFeedbackSwitchesOffWithoutAFinding(t *testing.T) {
	t.Parallel()
	dockerfile := "FROM alpine@" + digest + "\n"
	rec := &recorder{report: hook.ToolReport{Missing: []string{"hadolint 2.15.1 is not installed."}}}
	result := judge(t, written(t, "PostToolUse", "Dockerfile", dockerfile), map[string][]byte{"Dockerfile": []byte(dockerfile)}, rec, true)
	if result.Outcome != hook.Allow || len(result.Lines) != 0 || result.Notice != "" || len(rec.files) != 0 {
		t.Fatalf("result = %+v, tool files = %+v, want a plain allow", result, rec.files)
	}
}

// The switch leaves the certain checks on while the guard is installed
// (M6.6.4): on a post event they are notes, the only form Cursor reads.
func TestTheFeedbackSwitchKeepsTheCertainNotes(t *testing.T) {
	t.Parallel()
	script := "const assert = require(\"node:assert\");\n\nconst render = new Function(\"view\", \"return view.title;\");\n"
	rec := &recorder{}
	result := judge(t, written(t, "PostToolUse", "test/view/host.test.cjs", script), map[string][]byte{"test/view/host.test.cjs": []byte(script)}, rec, true)
	want := "test/view/host.test.cjs:3 surface.code-eval (javascript:S1523) runs text as code with new Function (Sonar S1523): export the code from a module and call it; writing it to a file and requiring it is the same execution."
	if result.Outcome != hook.Allow || len(result.Lines) != 1 || result.Lines[0] != want || len(rec.files) != 0 {
		t.Fatalf("result = %+v, tool files = %+v\nwant the one certain line %q and no tool", result, rec.files, want)
	}
}

func TestAPreWriteRunsNoToolAndFeedsNothingBack(t *testing.T) {
	t.Parallel()
	dockerfile := "FROM alpine@" + digest + "\n"
	rec := &recorder{}
	result := judge(t, written(t, "PreToolUse", "Dockerfile", dockerfile), nil, rec, false)
	if result.Outcome != hook.Allow || len(result.Lines) != 0 || len(rec.files) != 0 {
		t.Fatalf("result = %+v, tool files = %+v", result, rec.files)
	}
}
