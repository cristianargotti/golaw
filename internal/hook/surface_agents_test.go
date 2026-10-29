package hook_test

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/codex"
	"github.com/cristianargotti/golaw/internal/hook/cursor"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/surfacefix"
)

// The credential word, a literal value and the suppression marker, from
// halves so no scanner of the kit reads them here.
const (
	credentialWord = "pass" + "word"
	literal        = "zq81" + "-kept"
	marker         = "NO" + "SONAR"
	hexDigest      = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

// certain is one certain check: the file an agent writes over a base, its
// red and near contents, the place and message of the red finding, and its
// check id and Sonar key for the feedback line.
type certain struct {
	name, path, red, near string
	base                  map[string]string
	where, check, sonar   string
	message               string
}

func (c certain) reason() string { return c.where + " " + c.message }

func (c certain) line() string {
	name := c.check
	if c.sonar != "" {
		name += " (" + c.sonar + ")"
	}
	return c.where + " " + name + " " + c.message
}

func repeat(n int, text string) string { return strings.Repeat(text+"\n", n) }

func certainCases() []certain {
	properties := "sonar.projectKey=hub\nsonar.exclusions=**/*_test.go\n"
	return []certain{
		{
			name: "code-eval", path: "test/view/host.test.cjs",
			red:   "const assert = require(\"node:assert\");\n\nconst render = new Function(\"view\", \"return view.title;\");\nassert.strictEqual(render({ title: \"panel\" }), \"panel\");\n",
			near:  "const assert = require(\"node:assert\");\nconst { render } = require(\"../../views/render.cjs\");\n\nassert.strictEqual(render({ title: \"panel\" }), \"panel\");\n",
			where: "test/view/host.test.cjs:3", check: "surface.code-eval", sonar: "javascript:S1523",
			message: "runs text as code with new Function (Sonar S1523): export the code from a module and call it; writing it to a file and requiring it is the same execution.",
		},
		{
			name: "hardcoded-credential", path: "test/deploy/check_install.py",
			red:   "\"\"\"Install checks.\"\"\"\n\n\ndef test_admin():\n    " + credentialWord + " = \"" + literal + "\"\n    assert login(\"admin\", " + credentialWord + ")\n",
			near:  "\"\"\"Install checks.\"\"\"\n\nimport secrets\n\n\ndef test_admin():\n    secret = secrets.token_urlsafe(16)\n    assert login(\"admin\", secret)\n",
			where: "test/deploy/check_install.py:5", check: "surface.hardcoded-credential", sonar: "python:S2068",
			message: "assigns a literal to " + credentialWord + " (Sonar S2068): generate it at run time (secrets.token_urlsafe(16), crypto.randomUUID()) or read it from the environment.",
		},
		{
			name: "file-length", path: "scripts/smoke.sh",
			red: "#!/bin/sh\n" + repeat(311, "echo step"), near: "#!/bin/sh\n" + repeat(299, "echo step"),
			where: "scripts/smoke.sh", check: "surface.file-length",
			message: "has 312 lines, the limit is 300: split it by responsibility.",
		},
		{
			name: "inline-script-size", path: "views/panel.html",
			red:   repeat(11, "<p>row</p>") + "<script>\n" + repeat(129, "step();") + "</script>\n",
			near:  repeat(11, "<p>row</p>") + "<script type=\"module\" src=\"/views/panel.js\"></script>\n",
			where: "views/panel.html:12", check: "surface.inline-script-size",
			message: "holds an inline script of 129 lines, the limit is 40: move it to views/panel.js, embed it with go:embed and splice it when the view is served, so the linters read it.",
		},
		{
			name: "suppression", path: "views/common.js",
			red: repeat(7, "run();") + "done(); // " + marker + "\n", near: repeat(7, "run();") + "done(); // eslint-disable-line no-console -- the panel logs once\n",
			where: "views/common.js:8", check: "surface.suppression",
			message: "adds " + marker + ", which hides every rule on the line from the pipeline's analyzer: fix the finding; golaw lint names it.",
		},
		{
			name: "analyzer-widening", path: "sonar-project.properties", base: map[string]string{"sonar-project.properties": properties},
			red: "sonar.projectKey=hub\nsonar.exclusions=**/*_test.go,src/legacy/**\n", near: properties + "sonar.tests=test\n",
			where: "sonar-project.properties", check: "surface.analyzer-widening",
			message: "adds src/legacy/** to sonar.exclusions: an exclusion hides code from the pipeline's gate; fix the findings, or ask the person to change the scope.",
		},
		{
			name: "tag-and-digest", path: "Dockerfile",
			red:   "FROM valkey/valkey:8.1@sha256:" + hexDigest + "\nUSER 999:1000\n",
			near:  "# valkey/valkey:8.1\nFROM valkey/valkey@sha256:" + hexDigest + "\nUSER 999:1000\n",
			where: "Dockerfile:1", check: "docker.tag-and-digest", sonar: "docker:S8431",
			message: "writes FROM with a tag and a digest (Sonar S8431): keep only the digest and put the tag in a comment on the line above.",
		},
	}
}

func codexAdapter() hook.Adapter {
	return hook.Adapter{Parse: codex.Parse, Encode: codex.Encode, Oversized: codex.Oversized}
}

func cursorAdapter() hook.Adapter {
	return hook.Adapter{Parse: cursor.Parse, Encode: cursor.Encode, Oversized: cursor.Oversized}
}

// patched is the apply_patch payload of Codex that writes the content at
// the path, a file it adds or replaces.
func patched(t *testing.T, path, content string) []byte {
	t.Helper()
	lines := strings.SplitAfter(strings.TrimSuffix(content, "\n"), "\n")
	patch := make([]string, 0, len(lines)+3)
	patch = append(patch, "*** Begin Patch", "*** Add File: "+path)
	for _, line := range lines {
		patch = append(patch, "+"+strings.TrimSuffix(line, "\n"))
	}
	patch = append(patch, "*** End Patch")
	data, err := json.Marshal(map[string]any{
		"session_id": "session", "turn_id": "turn", "transcript_path": nil, "cwd": hooktest.Root,
		"hook_event_name": "PreToolUse", "model": "model", "permission_mode": "default",
		"tool_name": "apply_patch", "tool_input": map[string]any{"command": strings.Join(patch, "\n")}, "tool_use_id": "tool",
	})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func bytesOf(files map[string]string) map[string][]byte {
	out := make(map[string][]byte, len(files))
	for path, text := range files {
		out[path] = []byte(text)
	}
	return out
}

// before judges the pre event of an agent over the base.
func before(t *testing.T, agent string, adapter hook.Adapter, payload []byte, base map[string]string) hook.Result {
	t.Helper()
	root := hooktest.Repository(t)
	_, result := hook.Evaluate(t.Context(), agent, hooktest.Payload(payload, root), adapter, hooktest.Dependencies(t, root, bytesOf(base)))
	return result
}

func TestEveryCertainDenialDeniesBeforeTheWriteOnClaudeAndCodex(t *testing.T) {
	t.Parallel()
	shapes := map[string]struct {
		adapter hook.Adapter
		payload func(t *testing.T, path, content string) []byte
	}{
		"claude": {claudeAdapter(), func(t *testing.T, path, content string) []byte {
			t.Helper()
			return written(t, "PreToolUse", path, content)
		}},
		"codex": {codexAdapter(), patched},
	}
	for _, tc := range certainCases() {
		for agent, shape := range shapes {
			t.Run(tc.name+"/"+agent, func(t *testing.T) {
				t.Parallel()
				red := before(t, agent, shape.adapter, shape.payload(t, tc.path, tc.red), tc.base)
				if red.Outcome != hook.Deny || red.Reason != tc.reason() || red.CannotJudge ||
					!strings.HasPrefix(red.Human, "A operação foi bloqueada pela verificação "+tc.check+" no caminho "+tc.path+". ") {
					t.Fatalf("red = %+v\nwant the reason %q", red, tc.reason())
				}
				if near := before(t, agent, shape.adapter, shape.payload(t, tc.path, tc.near), tc.base); near.Outcome != hook.Allow {
					t.Fatalf("near = %+v, want an allow", near)
				}
			})
		}
	}
}

func TestCursorFeedsEveryCertainFindingBackAfterTheWrite(t *testing.T) {
	t.Parallel()
	pre := `{"hook_event_name":"preToolUse","conversation_id":"conversation","generation_id":"generation","workspace_roots":["/project"],"cwd":"/project","tool_name":"Write","tool_use_id":"tool","tool_input":{"__undocumented__":true}}`
	post := `{"hook_event_name":"postToolUse","conversation_id":"conversation","generation_id":"generation","workspace_roots":["/project"],"cwd":"/project","tool_name":"Write","tool_use_id":"tool","tool_input":{"__undocumented__":true},"tool_output":"{}","duration":1}`
	for _, tc := range certainCases() {
		if tc.check == "surface.analyzer-widening" {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := hooktest.Repository(t)
			first := hooktest.Dependencies(t, root, bytesOf(tc.base))
			after := map[string]string{}
			maps.Copy(after, tc.base)
			after[tc.path] = tc.red
			second := hooktest.Dependencies(t, root, bytesOf(after))
			second.State = first.State
			if _, captured := hook.Evaluate(t.Context(), "cursor", hooktest.Payload([]byte(pre), root), cursorAdapter(), first); captured.Outcome != hook.Allow {
				t.Fatalf("pre = %+v", captured)
			}
			event, result := hook.Evaluate(t.Context(), "cursor", hooktest.Payload([]byte(post), root), cursorAdapter(), second)
			if result.Outcome != hook.Allow || len(result.Lines) == 0 || !containsLine(result.Lines, tc.line()) {
				t.Fatalf("post = %+v\nwant the line %q", result, tc.line())
			}
			reply := cursorAdapter().Encode(event, result)
			if reply.ExitCode != 0 || !strings.Contains(string(reply.Stdout), "additional_context") {
				t.Fatalf("reply = %+v", reply)
			}
		})
	}
}

func containsLine(lines []string, want string) bool {
	return slices.Contains(lines, want)
}

func TestTheCorrectedTwinRaisesNoDenialOnClaudeOrCodex(t *testing.T) {
	t.Parallel()
	files, err := surfacefix.Files(surfacefix.Corrected)
	if err != nil {
		t.Fatal(err)
	}
	for path, data := range files {
		// The twin's Go files and its Dockerfile meet the Go and policy
		// families, which no native class concerns, and an agent creating the
		// analyzer scope meets the guard of check 6 by design; the native
		// silence of all three is proven by the twins test of
		// internal/checks/surface.
		if strings.HasSuffix(path, ".go") || path == "go.mod" || strings.HasSuffix(path, "Dockerfile") || path == "sonar-project.properties" {
			continue
		}
		for agent, adapter := range map[string]hook.Adapter{"claude": claudeAdapter(), "codex": codexAdapter()} {
			payload := written(t, "PreToolUse", path, string(data))
			if agent == "codex" {
				payload = patched(t, path, string(data))
			}
			if result := before(t, agent, adapter, payload, nil); result.Outcome == hook.Deny {
				t.Errorf("%s on %s = %+v", agent, path, result)
			}
		}
	}
}

// correctedCorpus is the near-miss corpus of M6.6.3: the corrected shapes
// the hand fixes of the hub wrote (survey 1.3), rewritten generically, and
// the shapes a localized view and a named suppression take.
func correctedCorpus() map[string]string {
	return map[string]string{
		"test/view/host.test.cjs":      "const assert = require(\"node:assert\");\nconst { render } = require(\"../../views/render.cjs\");\n\nassert.strictEqual(render({ title: \"panel\" }), \"panel\");\n",
		"views/render.cjs":             "function render(view) {\n  return view.title;\n}\n\nmodule.exports = { render };\n",
		"views/boot.js":                "const state = JSON.parse(document.getElementById(\"state\").textContent);\nsetTimeout(() => start(state), 0);\n",
		"views/labels.js":              "export const labels = { senha: \"Senha\", user: \"Usuário\" };\n",
		"views/common.js":              "run(); // eslint-disable-line no-console -- the panel logs once\n",
		"views/panel.html":             "<!doctype html>\n<html lang=\"pt-BR\">\n<head><title>Painel</title></head>\n<body>\n<output id=\"status\"></output>\n<script type=\"module\" src=\"/views/panel.js\"></script>\n<script>\nglobalThis.boot = true;\n</script>\n<script type=\"text/template\" id=\"row\">\n<tr><td>{{.Name}}</td></tr>\n</script>\n</body>\n</html>\n",
		"scripts/load.py":              "import ast\n\nvalue = ast.literal_eval(\"[1, 2]\")\n",
		"scripts/run.py":               "import subprocess  # noqa: S404 the runner passes a fixed argument list\n\nsubprocess.run([\"make\", \"ci\"], check=True)\n",
		"test/deploy/check_install.py": "import secrets\n\n\ndef test_admin():\n    secret = secrets.token_urlsafe(16)\n    assert login(\"admin\", secret)\n",
		"test/view/session.test.cjs":   "const { randomUUID } = require(\"node:crypto\");\n\nconst token = randomUUID();\nmodule.exports = { token };\n",
		"scripts/smoke.sh":             "#!/bin/sh\n" + repeat(299, "echo step"),
		"deploy/app.Dockerfile":        "# alpine:3.22\nFROM alpine@sha256:" + hexDigest + "\nARG DB_USER\nENV DB_USER=$DB_USER\nUSER 65532:65532\n",
	}
}

// certainIDs are the certain checks of A27.
func certainIDs() []string {
	return []string{"surface.code-eval", "surface.hardcoded-credential", "surface.file-length", "surface.inline-script-size", "surface.suppression", "surface.analyzer-widening", "docker.tag-and-digest"}
}

func TestCertainDenialsAllowTheCorrectedCorpus(t *testing.T) {
	t.Parallel()
	for path, content := range correctedCorpus() {
		for agent, adapter := range map[string]hook.Adapter{"claude": claudeAdapter(), "codex": codexAdapter()} {
			payload := written(t, "PreToolUse", path, content)
			if agent == "codex" {
				payload = patched(t, path, content)
			}
			result := before(t, agent, adapter, payload, nil)
			for _, id := range certainIDs() {
				if result.Outcome == hook.Deny && strings.Contains(result.Human, "verificação "+id+" ") {
					t.Errorf("%s on %s: %s denied a corrected shape: %+v", agent, path, id, result)
				}
			}
		}
	}
	if len(certainCases()) != len(certainIDs()) {
		t.Errorf("%d certain cases for the %d certain checks of A27", len(certainCases()), len(certainIDs()))
	}
}
