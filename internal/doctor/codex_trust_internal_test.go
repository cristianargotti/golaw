package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
)

// trustedCodex is Codex installed with its configuration directory beside
// the hooks file; extra opens config.toml, and with trusted the record holds
// the hashes /hooks would write for golaw's two handlers at their install
// position.
func trustedCodex(t *testing.T, extra string, trusted bool) installation {
	t.Helper()
	in := installCodex(t)
	in.input.ConfigDir = in.dir
	var config strings.Builder
	if _, writeErr := config.WriteString(extra); writeErr != nil {
		t.Fatal(writeErr)
	}
	events := []string{"PreToolUse", "PostToolUse"}
	if !trusted {
		events = nil
	}
	for _, event := range events {
		hash, err := install.CodexTrustHash(event, codexMatcher, json.RawMessage(codexHandler), runtime.GOOS)
		if err != nil {
			t.Fatal(err)
		}
		if _, writeErr := config.WriteString("\n[hooks.state.'" + install.CodexTrustKey(in.hooks, event, 0, 0) + "']\ntrusted_hash = '" + hash + "'\n"); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	writeFile(t, filepath.Join(in.dir, "config.toml"), config.String())
	return in
}

// codexTrusted is the trust and the trusted line of a Codex installation
// read from its record.
func codexTrusted(in installation) (trustFact, Line) {
	facts := collectAgent(in.input)
	facts.Trust = codexRecordTrust(in.input)
	return facts.Trust, trustedLines(facts)[0]
}

func TestCodexRecordTrustReadsEveryHandler(t *testing.T) {
	t.Parallel()
	in := trustedCodex(t, "", true)
	trust, line := codexTrusted(in)
	if trust.Source != sourceRecord || len(trust.Hooks) != 2 || line.State != stateOK || line.Code != codeTrusted ||
		!strings.Contains(line.Detail, "from the trust record in "+filepath.Join(in.dir, "config.toml")) {
		t.Fatalf("trust = %+v, line = %+v", trust, line)
	}
}

func TestCodexRecordTrustTellsChangedFromUntrusted(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, hooks, code, key string
	}{
		{"changed definition", strings.Replace(codexHooks(""), `"timeout":10`, `"timeout":11`, 1), codeRevalidation, ":pre_tool_use:0:0"},
		{"a group inserted before", codexHooks(`{"matcher":"Bash","hooks":[{"type":"command","command":"other-kit hook"}]}`), codeUntrusted, ":pre_tool_use:1:0"},
		{"handler removed", `{"hooks":{"PostToolUse":[{"matcher":"` + codexMatcher + `","hooks":[` + codexHandler + `]}]}}`, codeUntrusted, ":pre_tool_use:0:0"},
	} {
		in := trustedCodex(t, "", true)
		writeFile(t, in.hooks, tc.hooks)
		_, line := codexTrusted(in)
		if line.State != stateRed || line.Code != tc.code || !strings.Contains(line.Detail, tc.key) || !strings.Contains(line.Detail, "/hooks") {
			t.Errorf("%s: %+v", tc.name, line)
		}
	}
	if _, line := codexTrusted(trustedCodex(t, "", false)); line.Code != codeUntrusted || !strings.Contains(line.Detail, "not trusted, so Codex skips them") {
		t.Fatalf("no record = %+v", line)
	}
}

func TestCodexRecordTrustReadsTheFeatureAndTheProjectLayer(t *testing.T) {
	t.Parallel()
	if _, line := codexTrusted(trustedCodex(t, "[features]\nhooks = false\n", true)); line.Code != codeUntrusted || !strings.Contains(line.Detail, "the hooks feature is off") {
		t.Fatalf("feature off = %+v", line)
	}
	root := t.TempDir()
	for _, tc := range []struct {
		projects, state, code string
	}{
		{"[projects.'" + root + "']\ntrust_level = 'trusted'\n", stateOK, codeTrusted},
		{"[projects.'" + filepath.Dir(root) + "']\ntrust_level = 'trusted'\n", stateUnknown, codeTrustUnknown},
		{"[projects.'" + root + "']\ntrust_level = 'untrusted'\n", stateRed, codeUntrusted},
	} {
		project := trustedCodex(t, tc.projects, true)
		project.input.Scope, project.input.Root = scopeProject, root
		if _, line := codexTrusted(project); line.State != tc.state || line.Code != tc.code {
			t.Errorf("%q: %+v", tc.projects, line)
		}
	}
}

func TestCodexTrustLineStaysUnknownWithoutEvidence(t *testing.T) {
	t.Parallel()
	broken := trustedCodex(t, "[hooks\n", false)
	if trust, line := codexTrusted(broken); trust.Error == "" || line.Code != codeTrustUnknown || !strings.Contains(line.Detail, "cannot be read") {
		t.Fatalf("broken record = %+v", line)
	}
	absent := Input{Agent: agentCodex, Scope: "user", ReceiptPath: filepath.Join(t.TempDir(), "receipt.json")}
	if trust := codexRecordTrust(absent); trust.Source != "" {
		t.Fatalf("nothing installed = %+v", trust)
	}
	facts := collectAgent(absent)
	if line := trustedLines(facts)[0]; line.Code != codeTrustUnknown || line.Detail != "nothing is installed for Codex to trust" {
		t.Fatalf("nothing installed = %+v", line)
	}
	facts.Receipt.State = receiptRead
	if line := trustedLines(facts)[0]; line.Detail != "the Codex trust is not read" {
		t.Fatalf("not read = %+v", line)
	}
	facts.Trust = trustFact{Source: "codex app-server hooks/list", Hooks: []hookTrust{{Key: "k", Status: statusTrusted}}}
	if line := trustedLines(facts)[0]; line.Code != codeTrusted || !strings.HasSuffix(line.Detail, "(from codex app-server hooks/list)") {
		t.Fatalf("listed = %+v", line)
	}
	facts.Trust = trustFact{Source: sourceRecord, Config: "config.toml", Reason: "no observed environment", Hooks: []hookTrust{{Key: "k", Status: statusTrusted}}}
	if line := trustedLines(facts)[0]; !strings.Contains(line.Detail, "codex app-server did not answer: no observed environment") {
		t.Fatalf("fallback = %+v", line)
	}
}

func TestRecordStatusHashesTheCommandOfTheOperatingSystem(t *testing.T) {
	t.Parallel()
	handler := json.RawMessage(`{"type":"command","command":"'/opt/golaw' hook codex","commandWindows":"golaw.exe hook codex","timeout":10}`)
	entry := install.Entry{Event: "PreToolUse", Definition: handler}
	slots := map[string][]slot{"PreToolUse": {{Matcher: codexMatcher, Raw: handler}}}
	windows, err := install.CodexTrustHash("PreToolUse", codexMatcher, handler, goosWindows)
	if err != nil {
		t.Fatal(err)
	}
	trusted := map[string]string{install.CodexTrustKey("hooks.json", "PreToolUse", 0, 0): windows}
	if got := recordStatus("hooks.json", entry, slots, trusted, goosWindows); got.Status != statusTrusted {
		t.Fatalf("windows = %+v", got)
	}
	if got := recordStatus("hooks.json", entry, slots, trusted, "linux"); got.Status != statusModified {
		t.Fatalf("linux = %+v", got)
	}
}

// TestCodexRecordTrustRulesOutAHandlerTurnedOff turns golaw's PreToolUse
// handler off in hooks.state beside a matching hash and an observed
// dispatch, with no app-server to ask: Codex never runs it, so it is
// untrusted and the report never protecting (added by the review of the
// plan, not replayed).
func TestCodexRecordTrustRulesOutAHandlerTurnedOff(t *testing.T) {
	t.Parallel()
	in := trustedCodex(t, "", true)
	config := filepath.Join(in.dir, "config.toml")
	data, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, config, strings.Replace(string(data), "trusted_hash", "enabled = false\ntrusted_hash", 1))
	if _, line := codexTrusted(in); line.State != stateRed || line.Code != codeUntrusted || !strings.Contains(line.Detail, "turned off") || !strings.Contains(line.Detail, ":pre_tool_use:0:0") {
		t.Fatalf("a handler turned off = %+v", line)
	}
	in.input.Dispatch = Dispatch{State: DispatchObserved, OS: "linux"}
	if state := lineWith(Codex(t.Context(), in.input, &enrollmentStore{}).Lines, idState); state.Code == codeProtecting {
		t.Fatalf("a handler turned off is protecting: %+v", state)
	}
}

func TestCodexReportReadsItsTrust(t *testing.T) {
	t.Parallel()
	in := trustedCodex(t, "", true)
	in.input.Dispatch = Dispatch{State: DispatchObserved, OS: "linux"}
	report := Codex(t.Context(), in.input, &enrollmentStore{})
	if state := lineWith(report.Lines, idState); state.State != stateOK || state.Code != codeProtecting {
		t.Fatalf("state = %+v", report.Lines)
	}
}
