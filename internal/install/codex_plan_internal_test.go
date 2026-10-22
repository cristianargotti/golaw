package install

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
)

func TestCodexInstallScopesAndUpgrade(t *testing.T) {
	t.Parallel()
	for _, scope := range []string{"user", "project"} {
		t.Run(scope, func(t *testing.T) {
			t.Parallel()
			row := codexScopeSetup(t, scope)
			plan, err := PlanCodex(row.options)
			must(t, err)
			codexAssertFirstPlan(t, row, plan)
			codexAssertFirstApply(t, row, plan)
			codexAssertRepeat(t, row, plan)
			trusted := codexAssertTrusted(t, row, plan)
			codexAssertUpgrade(t, row, trusted)
		})
	}
}

// codexScopeRow is one scope of TestCodexInstallScopesAndUpgrade: its
// options and the files a Codex install of that scope reads and writes.
type codexScopeRow struct {
	scope                                       string
	options                                     Options
	directory, blockPath, hooksPath, configPath string
	config                                      []byte
}

func codexScopeSetup(t *testing.T, scope string) codexScopeRow {
	t.Helper()
	options := codexOptions(t, scope)
	directory := options.ConfigDir
	blockPath := filepath.Join(directory, "AGENTS.md")
	if scope == "project" {
		directory = filepath.Join(options.Root, ".codex")
		blockPath = filepath.Join(options.Root, "AGENTS.md")
	}
	hooksPath := filepath.Join(directory, "hooks.json")
	configPath := filepath.Join(options.ConfigDir, "config.toml")
	config := []byte("model = \"fixture\"\n")
	cinWrite(t, hooksPath, []byte(cinForeignHooks))
	cinWrite(t, blockPath, []byte("# Local\nKeep this.\n"))
	cinWrite(t, configPath, config)
	return codexScopeRow{
		scope: scope, options: options, directory: directory, blockPath: blockPath,
		hooksPath: hooksPath, configPath: configPath, config: config,
	}
}

func codexAssertFirstPlan(t *testing.T, row codexScopeRow, plan Plan) {
	t.Helper()
	if string(cinRead(t, row.hooksPath)) != cinForeignHooks {
		t.Fatal("planning changed hooks")
	}
	if _, statErr := os.Stat(plan.ReceiptPath); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("planning wrote receipt: %v", statErr)
	}
	if plan.Receipt.Agent != "codex" || plan.Receipt.HooksPath != row.hooksPath ||
		plan.Receipt.Bundle != filepath.Dir(plan.ReceiptPath) {
		t.Fatalf("receipt identity = %+v", plan.Receipt)
	}
	artifacts, _ := codexRulesExpectation(row)
	if len(plan.Receipt.Entries) != 2 || len(plan.Receipt.Blocks) != 1 ||
		len(plan.Receipt.Artifacts) != artifacts {
		t.Fatalf("receipt ownership = %+v", plan.Receipt)
	}
}

func codexRulesExpectation(row codexScopeRow) (int, string) {
	artifacts, rulesPath := 1, filepath.Join(row.options.ConfigDir, "rules", "golaw.rules")
	if row.scope == "project" {
		artifacts, rulesPath = 2, filepath.Join(row.directory, "rules", "golaw.rules")
	}
	return artifacts, rulesPath
}

func codexAssertFirstApply(t *testing.T, row codexScopeRow, plan Plan) {
	t.Helper()
	var diff bytes.Buffer
	must(t, Apply(t.Context(), plan, &diff, cinEnrollment{}))
	if diff.Len() == 0 {
		t.Fatal("first installation printed no diff")
	}
	codexAssertForeign(t, cinRead(t, row.hooksPath))
	codexAssertEntries(t, row.options, plan)
	_, rulesPath := codexRulesExpectation(row)
	if _, statErr := os.Stat(rulesPath); (row.scope == "project") == errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("rules file at %s scope: %v", row.scope, statErr)
	}
	if !bytes.Equal(cinRead(t, row.configPath), row.config) {
		t.Fatal("installation changed trust configuration")
	}
	skill, readErr := assets.Read("agents/SKILL.md")
	must(t, readErr)
	if !bytes.Equal(cinRead(t, SkillPath(row.options)), skill) {
		t.Fatal("installed skill differs from source")
	}
	for _, artifact := range plan.Receipt.Artifacts {
		if Digest(cinRead(t, artifact.Path)) != artifact.SHA256 {
			t.Fatalf("artifact digest differs: %s", artifact.Path)
		}
	}
}

func codexAssertRepeat(t *testing.T, row codexScopeRow, plan Plan) {
	t.Helper()
	before := cinRead(t, plan.ReceiptPath)
	second, secondErr := PlanCodex(row.options)
	must(t, secondErr)
	if len(second.Changes) != 0 ||
		!strings.Contains(strings.Join(second.Notes, "\n"), "/hooks") {
		t.Fatalf("repeat plan = %+v", second)
	}
	var diff bytes.Buffer
	must(t, Apply(t.Context(), second, &diff, cinEnrollment{}))
	if diff.Len() != 0 || !bytes.Equal(before, cinRead(t, plan.ReceiptPath)) {
		t.Fatal("repeat installation changed receipt or printed a diff")
	}
}

func codexAssertTrusted(t *testing.T, row codexScopeRow, plan Plan) string {
	t.Helper()
	records := make([]string, 0, 1+len(plan.Receipt.Entries))
	records = append(records, string(row.config))
	for _, entry := range plan.Receipt.Entries {
		hash, hashErr := CodexTrustHash(
			entry.Event, entry.Matcher, entry.Definition, row.options.GOOS)
		must(t, hashErr)
		key := CodexTrustKey(row.hooksPath, entry.Event, entry.Group, entry.Handler)
		records = append(records, fmt.Sprintf("\n[hooks.state.%q]\ntrusted_hash = %q\n", key, hash))
	}
	trusted := strings.Join(records, "")
	cinWrite(t, row.configPath, []byte(trusted))
	known, knownErr := PlanCodex(row.options)
	must(t, knownErr)
	if strings.Contains(strings.Join(known.Notes, "\n"), "/hooks") {
		t.Fatalf("unchanged trusted definition needs validation: %v", known.Notes)
	}
	if row.scope == "project" &&
		!strings.Contains(strings.Join(known.Notes, "\n"), "projeto") {
		t.Fatalf("missing project trust instruction: %v", known.Notes)
	}
	return trusted
}

func codexAssertUpgrade(t *testing.T, row codexScopeRow, trusted string) {
	t.Helper()
	options := row.options
	options.Executable += "-next"
	upgraded := codexInstall(t, options)
	if !strings.Contains(strings.Join(upgraded.Notes, "\n"), "/hooks") {
		t.Fatal("upgrade omitted validation instruction")
	}
	codexAssertForeign(t, cinRead(t, row.hooksPath))
	codexAssertEntries(t, options, upgraded)
	if string(cinRead(t, row.configPath)) != trusted {
		t.Fatal("upgrade changed trust configuration")
	}
	codexOlderSkill(t, options, upgraded.ReceiptPath)
	refreshed, refreshErr := PlanCodex(options)
	must(t, refreshErr)
	if len(refreshed.Changes) != 2 || refreshed.Changes[0].Path != SkillPath(options) || refreshed.Changes[1].Path != refreshed.ReceiptPath {
		t.Fatalf("an older skill the receipt recorded = %+v", refreshed.Changes)
	}
}

// codexOlderSkill puts the skill an earlier kit wrote in place, with the
// receipt recording it, as an install of that kit left them.
func codexOlderSkill(t *testing.T, options Options, receiptPath string) {
	t.Helper()
	older := []byte("An older golaw skill.\n")
	cinWrite(t, SkillPath(options), older)
	receipt, err := ReadReceipt(receiptPath)
	must(t, err)
	for index := range receipt.Artifacts {
		if receipt.Artifacts[index].Path == SkillPath(options) {
			receipt.Artifacts[index].SHA256 = Digest(older)
		}
	}
	cinWrite(t, receiptPath, codexJSON(t, receipt))
}

func codexAssertForeign(t *testing.T, data []byte) {
	t.Helper()
	before := codexDecode(t, []byte(cinForeignHooks))
	after := codexDecode(t, data)
	if after["description"] != before["description"] {
		t.Fatal("foreign description changed")
	}
	original, ok := before["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("original hooks is not a mapping: %T", before["hooks"])
	}
	current, ok := after["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("current hooks is not a mapping: %T", after["hooks"])
	}
	for _, event := range []string{"PreToolUse", "PostToolUse", "Stop"} {
		oldGroups, ok := original[event].([]any)
		if !ok || len(oldGroups) == 0 {
			t.Fatalf("original %s is not a nonempty group list", event)
		}
		newGroups, ok := current[event].([]any)
		if !ok || len(newGroups) == 0 {
			t.Fatalf("current %s is not a nonempty group list", event)
		}
		if !reflect.DeepEqual(newGroups[0], oldGroups[0]) {
			t.Fatalf("foreign %s group changed", event)
		}
	}
}

func codexAssertEntries(t *testing.T, options Options, plan Plan) {
	t.Helper()
	document := codexDecode(t, cinRead(t, plan.Receipt.HooksPath))
	events, ok := document["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("hooks is not a mapping: %T", document["hooks"])
	}
	seen := make(map[string]int)
	for _, entry := range plan.Receipt.Entries {
		seen[entry.Event]++
		codexAssertEntry(t, options, events, entry)
	}
	if seen["PreToolUse"] != 1 || seen["PostToolUse"] != 1 {
		t.Fatalf("owned events = %v", seen)
	}
}

func codexAssertEntry(t *testing.T, options Options, events map[string]any, entry Entry) {
	t.Helper()
	if entry.Group != 1 || entry.Handler != 0 ||
		entry.Matcher != "^(Bash|apply_patch)$" {
		t.Fatalf("owned position or matcher = %+v", entry)
	}
	groups, ok := events[entry.Event].([]any)
	if !ok {
		t.Fatalf("%s is not a group list", entry.Event)
	}
	if len(groups) != 2 {
		t.Fatalf("%s groups = %d", entry.Event, len(groups))
	}
	group, ok := groups[entry.Group].(map[string]any)
	if !ok {
		t.Fatalf("%s group is not a mapping", entry.Event)
	}
	handlers, ok := group["hooks"].([]any)
	if !ok || len(handlers) <= entry.Handler {
		t.Fatalf("%s hooks is not a sufficient handler list", entry.Event)
	}
	handler, ok := handlers[entry.Handler].(map[string]any)
	if !ok {
		t.Fatalf("%s handler is not a mapping", entry.Event)
	}
	if handler["command"] != HookCommand(options.Executable, "linux", "hook", "codex") ||
		handler["type"] != "command" || handler["timeout"] != float64(10) {
		t.Fatalf("installed handler = %#v", handler)
	}
	if !reflect.DeepEqual(handler, codexDecode(t, entry.Definition)) {
		t.Fatal("receipt definition differs from installed handler")
	}
}

func TestCodexInstallRejectsStalePlan(t *testing.T) {
	t.Parallel()
	options := codexOptions(t, "user")
	plan, err := PlanCodex(options)
	must(t, err)
	cinWrite(t, plan.Receipt.HooksPath, []byte(`{"description":"new","hooks":{}}`))
	if err := Apply(t.Context(), plan, io.Discard, cinEnrollment{}); !errors.Is(err, ErrChanged) {
		t.Fatalf("stale plan = %v", err)
	}
}
