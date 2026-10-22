package doctor

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
)

// errUnanswered is the failure of a lister double.
var errUnanswered = errors.New("codex app-server did not start")

// listerDouble answers hooks/list with fixed hooks or an error and counts
// the times it was asked.
type listerDouble struct {
	hooks []listedHook
	err   error
	asked int
}

func (l *listerDouble) ListHooks(context.Context, AgentEnvironment, string) ([]listedHook, error) {
	l.asked++
	return l.hooks, l.err
}

// listedAs lists golaw's two handlers of an installation with a status.
func listedAs(in installation, status string, enabled bool) []listedHook {
	return []listedHook{
		{Key: install.CodexTrustKey(in.hooks, "PreToolUse", 0, 0), SourcePath: in.hooks, TrustStatus: status, Enabled: enabled},
		{Key: install.CodexTrustKey(in.hooks, "PostToolUse", 0, 0), SourcePath: in.hooks, TrustStatus: status, Enabled: enabled},
	}
}

func TestCodexListedTrustMapsTheVerdictOfCodex(t *testing.T) {
	t.Parallel()
	in := trustedCodex(t, "", false)
	for _, tc := range []struct {
		name    string
		status  string
		enabled bool
		code    string
	}{
		{"trusted", statusTrusted, true, codeTrusted},
		{"managed", statusManaged, true, codeTrusted},
		{"modified", statusModified, true, codeRevalidation},
		{"untrusted", statusUntrusted, true, codeUntrusted},
		{"disabled", statusTrusted, false, codeUntrusted},
	} {
		facts := collectAgent(in.input)
		facts.Trust = codexListedTrust(in.input, listedAs(in, tc.status, tc.enabled))
		if line := trustedLines(facts)[0]; line.Code != tc.code || !strings.Contains(line.Detail, "from "+sourceHooksList) {
			t.Errorf("%s: %+v", tc.name, line)
		}
	}
	foreign := listedAs(in, statusTrusted, true)
	foreign[0].SourcePath = filepath.Join(in.dir, "other.json")
	if trust := codexListedTrust(in.input, foreign); trust.Hooks[0].Status != statusUntrusted || trust.Hooks[1].Status != statusTrusted {
		t.Fatalf("another file at the same position = %+v", trust.Hooks)
	}
}

func TestCodexListedTrustReadsAnUnlistedProjectAsUntrusted(t *testing.T) {
	t.Parallel()
	in := trustedCodex(t, "", false)
	in.input.Scope, in.input.Root = scopeProject, t.TempDir()
	facts := collectAgent(in.input)
	facts.Trust = codexListedTrust(in.input, nil)
	if line := trustedLines(facts)[0]; line.Code != codeUntrusted || !strings.Contains(line.Detail, "only once the project is trusted") {
		t.Fatalf("unlisted project = %+v", line)
	}
	writeFile(t, in.hooks, `{"hooks":{}}`)
	if trust := codexListedTrust(in.input, nil); trust.Hooks[0].Status != statusAbsent {
		t.Fatalf("a removed handler = %+v", trust.Hooks)
	}
}

func TestCodexTrustAsksCodexFirstAndNamesTheFallback(t *testing.T) {
	t.Parallel()
	in := trustedCodex(t, "", true)
	answered := &listerDouble{hooks: listedAs(in, statusModified, true)}
	if trust := codexTrust(t.Context(), in.input, answered); trust.Source != sourceHooksList || trust.Hooks[0].Status != statusModified || answered.asked != 1 {
		t.Fatalf("answered = %+v", trust)
	}
	silent := &listerDouble{err: errUnanswered}
	trust := codexTrust(t.Context(), in.input, silent)
	if trust.Source != sourceRecord || trust.Reason != errUnanswered.Error() || trust.Hooks[0].Status != statusTrusted {
		t.Fatalf("fallback = %+v", trust)
	}
	absent := &listerDouble{}
	if trust := codexTrust(t.Context(), Input{ReceiptPath: filepath.Join(t.TempDir(), "receipt.json")}, absent); trust.Source != "" || absent.asked != 0 {
		t.Fatalf("nothing installed started the server: %+v, asked %d", trust, absent.asked)
	}
}

func TestCodexReportAsksCodexThroughTheObservedPath(t *testing.T) {
	t.Parallel()
	in := trustedCodex(t, "", false)
	dir := doubleNamed(t, agentCodex)
	hooks := `{"key":"` + jsonText(install.CodexTrustKey(in.hooks, "PreToolUse", 0, 0)) + `","sourcePath":"` + jsonText(in.hooks) + `","trustStatus":"trusted","enabled":true},` +
		`{"key":"` + jsonText(install.CodexTrustKey(in.hooks, "PostToolUse", 0, 0)) + `","sourcePath":"` + jsonText(in.hooks) + `","trustStatus":"trusted","enabled":true}`
	writeFile(t, filepath.Join(dir, hooksListFixture), listResult(hooks))
	in.input.Environment = AgentEnvironment{Source: sourceAgent, Path: dir, PathExt: ".EXE"}
	in.input.Root = t.TempDir()
	line := lineWith(Codex(t.Context(), in.input, &enrollmentStore{}).Lines, idTrusted)
	if line.State != stateOK || !strings.Contains(line.Detail, "from "+sourceHooksList) {
		t.Fatalf("trusted = %+v", line)
	}
}

// jsonText escapes a path for a JSON string, backslashes of Windows
// included.
func jsonText(value string) string {
	return strings.ReplaceAll(value, `\`, `\\`)
}
