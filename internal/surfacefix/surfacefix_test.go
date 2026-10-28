package surfacefix_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/surfacefix"
)

func files(t *testing.T, twin surfacefix.Twin) map[string][]byte {
	t.Helper()
	got, err := surfacefix.Files(twin)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// TestEveryIssueClassIsInTheRedTwinAndGoneFromTheCorrectedOne reads the
// token that carries each hub issue class of M6.10 row (1): the red twin
// holds it, the corrected twin does not.
func TestEveryIssueClassIsInTheRedTwinAndGoneFromTheCorrectedOne(t *testing.T) {
	t.Parallel()
	red, corrected := files(t, surfacefix.Red), files(t, surfacefix.Corrected)
	tests := map[string]struct {
		path  string
		token string
	}{
		"an unbraced multiline block (S2681)":       {path: "views/panel.html", token: "if (rows.length === 0)\n    state.textContent"},
		"a nested ternary (S3358)":                  {path: "views/panel.html", token: `? "empty" : count > 9 ?`},
		"window (S7764)":                            {path: "views/panel.html", token: "window.addEventListener"},
		"an extra argument (S930)":                  {path: "views/panel.html", token: `render({ title: "panel" }, state)`},
		"a duplicate selector (css:S4666)":          {path: "views/panel.html", token: "table { width: 100%; }"},
		"role status in markup (Web:S6819)":         {path: "views/panel.html", token: `<div role="status"`},
		"an unused import (S1128)":                  {path: "views/panel.jsx", token: `import { useState } from "react";`},
		"an index key (S6479)":                      {path: "views/panel.jsx", token: "key={index}"},
		"new Function in a test (S1523)":            {path: "test/view/host.test.cjs", token: "new Function("},
		"vm.runInThisContext in a test (S1523)":     {path: "test/view/host.test.cjs", token: "vm.runInThisContext("},
		"a literal credential in a test (S2068)":    {path: "test/deploy/test_install.py", token: "password = \""},
		"a credential in a URL (S2068)":             {path: "test/deploy/test_install.py", token: "admin:hunt" + "er2-db@"},
		"a dict comprehension of None (S7519)":      {path: "test/deploy/test_install.py", token: "{name: None for name in"},
		"a tag beside a digest (docker:S8431)":      {path: "Dockerfile", token: "golang:1.27.3-trixie@sha256:"},
		"an unquoted expansion (shellcheck)":        {path: "scripts/smoke.sh", token: "-o $target"},
		"a final stage without USER (docker:S6471)": {path: "deploy/cache.Dockerfile", token: "COPY cache.conf /etc/cache.conf\nENTRYPOINT"},
		"a bare except in a test (python:S5754)":    {path: "test/deploy/test_install.py", token: "    except:\n"},
		"eval in an HTML test page (S1523)":         {path: "test/view/page.html", token: "eval(expression)"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if !bytes.Contains(red[tc.path], []byte(tc.token)) {
				t.Errorf("red %s lacks %q", tc.path, tc.token)
			}
			if bytes.Contains(corrected[tc.path], []byte(tc.token)) {
				t.Errorf("corrected %s still holds %q", tc.path, tc.token)
			}
		})
	}
	for _, fix := range []string{"aws:SecureTransport", "LoggingConfiguration"} {
		if bytes.Contains(red["infra/10-base.yml"], []byte(fix)) || !bytes.Contains(corrected["infra/10-base.yml"], []byte(fix)) {
			t.Errorf("only the corrected template holds %s (S6249, S6258)", fix)
		}
	}
}

func TestEscapedFilesAreDecodedAndRenamed(t *testing.T) {
	t.Parallel()
	for _, twin := range []surfacefix.Twin{surfacefix.Red, surfacefix.Corrected} {
		got := files(t, twin)
		for path := range got {
			if strings.HasSuffix(path, ".esc") {
				t.Errorf("twin %d keeps the escaped name %s", twin, path)
			}
		}
		if !bytes.HasPrefix(got["go.mod"], []byte("module example.com/hub\n")) {
			t.Errorf("twin %d go.mod = %q, want the module line", twin, got["go.mod"])
		}
		if !bytes.Contains(got["sonar-project.properties"], []byte("sonar.exclusions=**/*_test.go,integration/**")) {
			t.Errorf("twin %d lacks the analyzer exclusions", twin)
		}
	}
	if got := files(t, surfacefix.Red)["test/deploy/test_install.py"]; bytes.Contains(got, []byte(`\u00`)) {
		t.Errorf("an escape survived the decoding: %q", got)
	}
}

func TestNoTwinCarriesATermTheKitForbids(t *testing.T) {
	t.Parallel()
	for _, twin := range []surfacefix.Twin{surfacefix.Red, surfacefix.Corrected} {
		for path, content := range files(t, twin) {
			for _, term := range []string{string(rune(0x2014)), string(rune(0x2013)), "/" + "Users/"} {
				if bytes.Contains(content, []byte(term)) {
					t.Errorf("twin %d %s holds %q", twin, path, term)
				}
			}
		}
	}
}

func TestExpectationsNameTheRedTwinOnly(t *testing.T) {
	t.Parallel()
	red, corrected := files(t, surfacefix.Red), files(t, surfacefix.Corrected)
	classes := map[string]bool{}
	for _, e := range surfacefix.Expectations() {
		classes[e.Class] = true
		if _, ok := red[e.Path]; !ok {
			t.Errorf("%+v names a path the red twin lacks", e)
		}
		if !model.ValidID(e.Check) || !model.ValidCode(e.Code) || e.Count < 1 {
			t.Errorf("%+v is not a check id, a finding code and a count", e)
		}
		if e.Path == "views/panel.html" && e.Code == "inline-script-size" {
			if _, moved := corrected["views/panel.js"]; !moved {
				t.Error("the corrected twin does not move the inline script into a file")
			}
		}
	}
	for _, class := range []string{"html-inline-script", "jsx-list", "cjs-eval", "html-style", "python-test", "dockerfile", "cloudformation", "shell"} {
		if !classes[class] {
			t.Errorf("no expectation for the class %s", class)
		}
	}
}

func TestWriteMaterializesATwinIntoAnEmptyDirectoryOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := surfacefix.Write(dir, surfacefix.Red); err != nil {
		t.Fatal(err)
	}
	for path, want := range files(t, surfacefix.Red) {
		got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(path)))
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s = %q, %v", path, got, err)
		}
	}
	if err := surfacefix.Write(dir, surfacefix.Corrected); !errors.Is(err, surfacefix.ErrNotEmpty) {
		t.Errorf("a second Write into the same directory = %v, want ErrNotEmpty", err)
	}
	if _, err := surfacefix.Files(surfacefix.Twin(9)); err == nil {
		t.Error("an unknown twin is refused")
	}
}
