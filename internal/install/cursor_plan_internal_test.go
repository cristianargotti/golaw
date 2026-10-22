package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
)

func TestCursorPlanScopes(t *testing.T) {
	t.Parallel()
	for _, scope := range []string{"user", "project"} {
		for _, target := range []string{"darwin", "linux", "windows"} {
			t.Run(scope+"/"+target, func(t *testing.T) {
				t.Parallel()
				cursorPlanScope(t, scope, target)
			})
		}
	}
}

func cursorPlanScope(t *testing.T, scope, target string) {
	t.Helper()
	options := cursorOptions(t, scope)
	options.GOOS = target
	config := options.ConfigDir
	if scope == "project" {
		config = filepath.Join(options.Root, ".cursor")
	}
	path := filepath.Join(config, "hooks.json")
	foreign := cursorRead(t, "testdata/cursor_foreign.json")
	must(t, WriteAtomic(path, foreign))
	plan, err := PlanCursor(options)
	must(t, err)
	cursorCheckPlan(t, options, plan)
	cursorCheckPlanning(t, path, foreign, plan.ReceiptPath)
	enrollment := cursorApply(t, plan)
	cursorCheckEnrollment(t, enrollment, options, 1)
	cursorCheckReceipt(t, options, plan)
	cursorCheckHandlers(t, options, plan)
	cursorCheckFiles(t, options, config)
	second, err := PlanCursor(options)
	must(t, err)
	if len(second.Changes) != 0 {
		t.Fatalf("second install has %d changes", len(second.Changes))
	}
}

func cursorCheckPlan(t *testing.T, options Options, plan Plan) {
	t.Helper()
	_, identity, err := ProjectIdentity(options.Root)
	must(t, err)
	path := filepath.Join(options.StateDir, "cursor", "user", "receipt.json")
	if options.Scope == "project" {
		path = filepath.Join(options.StateDir, "cursor", "project", identity, "receipt.json")
	}
	if plan.ReceiptPath != path || plan.Receipt.ProjectID != identity {
		t.Fatalf("receipt path = %q, identity = %q", plan.ReceiptPath, plan.Receipt.ProjectID)
	}
	if plan.PolicyDigest != options.PolicyDigest || plan.ProfileDigest != options.ProfileDigest {
		t.Fatalf("plan digests = %q, %q", plan.PolicyDigest, plan.ProfileDigest)
	}
}

func cursorCheckPlanning(t *testing.T, path string, before []byte, receipt string) {
	t.Helper()
	if !bytes.Equal(cursorRead(t, path), before) {
		t.Fatal("planning wrote hooks")
	}
	if _, err := os.Stat(receipt); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("planning wrote receipt: %v", err)
	}
}

func cursorCheckReceipt(t *testing.T, options Options, plan Plan) {
	t.Helper()
	receipt, err := ReadReceipt(plan.ReceiptPath)
	must(t, err)
	if receipt.Agent != "cursor" || receipt.Scope != options.Scope ||
		receipt.Bundle != filepath.Dir(plan.ReceiptPath) || len(receipt.Entries) != 5 {
		t.Fatalf("receipt = %+v", receipt)
	}
	want := 1
	if options.Scope == "project" {
		want = 2
	}
	if len(receipt.Artifacts) != want {
		t.Fatalf("artifacts = %+v", receipt.Artifacts)
	}
	for _, artifact := range receipt.Artifacts {
		if Digest(cursorRead(t, artifact.Path)) != artifact.SHA256 ||
			artifact.Path == receipt.HooksPath {
			t.Fatalf("artifact ownership = %+v", artifact)
		}
	}
	note := "O Cursor é uma prévia de compatibilidade: as escritas dele são capturadas, mas ainda não bloqueadas (veja docs/agent-contracts.md)"
	if len(plan.Notes) != 1 || plan.Notes[0] != note {
		t.Fatalf("notes = %q", plan.Notes)
	}
}

func cursorCheckHandlers(t *testing.T, options Options, plan Plan) {
	t.Helper()
	handlers := cursorHandlers(t, plan.Receipt.HooksPath)
	for index, want := range []string{"first-check", "second-check"} {
		var foreign map[string]any
		must(t, json.Unmarshal(handlers["preToolUse"][index], &foreign))
		if foreign["command"] != want {
			t.Fatalf("foreign order = %#v", foreign)
		}
	}
	for _, entry := range plan.Receipt.Entries {
		cursorCheckHandler(t, options, entry, handlers[entry.Event][entry.Group])
	}
	if len(handlers["afterFileEdit"]) != 0 {
		t.Fatal("unsupported edit event installed")
	}
}

func cursorCheckHandler(t *testing.T, options Options, entry Entry, raw json.RawMessage) {
	t.Helper()
	var handler struct {
		Command    string `json:"command"`
		Matcher    string `json:"matcher"`
		Timeout    int    `json:"timeout"`
		FailClosed bool   `json:"failClosed"`
	}
	must(t, json.Unmarshal(raw, &handler))
	binary, ok := CommandBinary(handler.Command)
	if !ok || binary != options.Executable || handler.Timeout != 10 ||
		!handler.FailClosed || entry.Handler != 0 {
		t.Fatalf("handler = %+v, entry = %+v", handler, entry)
	}
	if entry.Event == "preToolUse" && handler.Matcher != "^(Write|Delete|Shell)$" {
		t.Fatalf("pre matcher = %q", handler.Matcher)
	}
	left, err := cursorCompact(raw)
	must(t, err)
	right, err := cursorCompact(entry.Definition)
	must(t, err)
	if !bytes.Equal(left, right) {
		t.Fatal("receipt definition differs from installed definition")
	}
}

func cursorCheckFiles(t *testing.T, options Options, config string) {
	t.Helper()
	skill, err := assets.Read("agents/SKILL.md")
	must(t, err)
	if !bytes.Equal(cursorRead(t, SkillPath(options)), skill) {
		t.Fatal("skill copy differs")
	}
	path := filepath.Join(config, "rules", "golaw.mdc")
	if options.Scope == "user" {
		if _, statErr := os.Stat(path); !errors.Is(statErr, fs.ErrNotExist) {
			t.Fatalf("user rule exists: %v", statErr)
		}
		return
	}
	rule := cursorRead(t, path)
	source, err := assets.Read("agents/cursor/golaw.mdc")
	must(t, err)
	if !bytes.Equal(rule, source) || !bytes.Contains(rule, []byte("alwaysApply: true")) ||
		len(strings.Split(strings.TrimSuffix(string(rule), "\n"), "\n")) > 20 {
		t.Fatal("project rule violates its source or budget")
	}
}
