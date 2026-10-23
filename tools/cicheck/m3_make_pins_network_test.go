//go:build network

package cicheck_test

import (
	"encoding/json"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
)

// TestM3MakePinsResolve resolves through the module proxy every go run pin
// of the golaw.mk every capability renders, but the kit's own tools, whose
// version is the synthetic pin of the rendering: a pin no proxy serves
// fails an adopter's make ci on its first run.
func TestM3MakePinsResolve(t *testing.T) {
	t.Parallel()
	tools := make(map[string]string)
	for _, tool := range runner.Toolchain() {
		tools[tool.Name] = tool.Version
	}
	rendered, err := assets.RenderAllContext(golaw.FS, assets.Context{
		CIBackend: "none", ModulePath: "example.test/service", GoVersion: "1.27", KitVersion: "0.2.0", Language: model.DefaultLanguage,
		Capabilities: model.Capabilities(), Decisions: "docs/adr", Main: "root", Tools: tools,
	})
	if err != nil {
		t.Fatal(err)
	}
	var body string
	for _, file := range rendered {
		if file.Path == "golaw.mk" {
			body = string(file.Data)
		}
	}
	pins := regexp.MustCompile(`go run ([a-z0-9./_-]+)@(v\d+\.\d+\.\d+)`).FindAllStringSubmatch(body, -1)
	resolved := 0
	for _, pin := range pins {
		if strings.HasPrefix(pin[1], "github.com/cristianargotti/golaw/") {
			continue
		}
		if reason := m3ResolveModule(t, pin[1], pin[2]); reason != "" {
			t.Errorf("%s@%s: %s", pin[1], pin[2], reason)
		}
		resolved++
	}
	if resolved < 3 {
		t.Fatalf("golaw.mk names %d external go run pins, want oasdiff, go-licenses and gremlins: %q", resolved, pins)
	}
}

// m3ResolveModule finds the module that holds a package path at a version,
// trying each prefix from the longest, and answers why the proxy does not
// serve that exact version, or nothing when it does.
func m3ResolveModule(t *testing.T, pkg, version string) string {
	t.Helper()
	for prefix := pkg; prefix != "." && prefix != "/"; prefix = path.Dir(prefix) {
		cmd := exec.CommandContext(t.Context(), "go", "list", "-m", "-json", prefix+"@"+version)
		cmd.Dir = t.TempDir()
		cmd.Env = append(cmd.Environ(), "GOFLAGS=", "GOWORK=off")
		out, err := cmd.Output()
		if err != nil {
			continue
		}
		var module struct{ Path, Version string }
		if decodeErr := json.Unmarshal(out, &module); decodeErr != nil {
			return "the module query answers no JSON: " + decodeErr.Error()
		}
		if module.Version != version {
			return "the proxy answers " + module.Path + " " + module.Version
		}
		return ""
	}
	return "no module of the path serves that version"
}
