package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
)

// renderedGolawMK is golaw.mk as init renders it for a Go repository.
func renderedGolawMK(t *testing.T) string {
	t.Helper()
	tools := make(map[string]string)
	for _, tool := range runner.Toolchain() {
		tools[tool.Name] = tool.Version
	}
	rendered, err := assets.RenderAllContext(golaw.FS, assets.Context{
		CIBackend: "none", ModulePath: newCodeModule, GoVersion: "1.27", KitVersion: testVersion, Language: model.DefaultLanguage,
		Capabilities: []string{"go"}, Decisions: "docs/adr", Main: bootstrap.MainRoot, Tools: tools,
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := range rendered {
		if rendered[i].Path == "golaw.mk" {
			return string(rendered[i].Data)
		}
	}
	t.Fatal("init renders no golaw.mk")
	return ""
}

// recipe is what make would run for coverage-new, printed and not run.
func recipe(t *testing.T, dir string, vars ...string) []string {
	t.Helper()
	out, err := runner.Run(t.Context(), runner.Command{
		Name: "make", Args: append([]string{"-n", "-f", "golaw.mk", "coverage-new"}, vars...), Dir: dir,
	})
	if errors.Is(err, runner.ErrStart) {
		t.Skip("make is not installed here: the coverage-new recipe is proven on the runners that hold it")
	}
	if err != nil || out.Code != 0 {
		t.Fatalf("make -n coverage-new = %d, %v, %s", out.Code, err, out.Stderr)
	}
	return strings.Split(strings.TrimSpace(string(out.Stdout)), "\n")
}

func assertJudgeStep(t *testing.T, line, wantBase string) {
	t.Helper()
	fields := strings.Fields(line)
	if len(fields) < 2 || fields[0] != "golaw" || fields[1] != "coverage" {
		t.Fatalf("the judge step = %q, want golaw coverage", line)
	}
	cmd := newTestApp(t).coverageCommand()
	if err := cmd.ParseFlags(fields[2:]); err != nil {
		t.Fatalf("golaw coverage refuses the recipe %q: %v", line, err)
	}
	newCode, newCodeErr := cmd.Flags().GetBool("new-code")
	input, inputErr := cmd.Flags().GetString("input")
	base, baseErr := cmd.Flags().GetString("base")
	if err := errors.Join(newCodeErr, inputErr, baseErr); err != nil {
		t.Fatal(err)
	}
	if !newCode || input != "profile.cov" || cmd.Flags().Changed("base") && base == "" || wantBase != "" && base != wantBase {
		t.Fatalf("the judge step = %q: new code %v, input %q, base %q", line, newCode, input, base)
	}
}

func TestCoverageNewTargetWritesAndReadsThePipelineReport(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"golaw.mk": renderedGolawMK(t)})
	for _, tc := range []struct {
		name string
		vars []string
		base string
	}{
		{"near-the-default-base", nil, ""},
		{"near-a-named-base", []string{"COVERAGE_BASE=release/base"}, "release/base"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			lines := recipe(t, dir, tc.vars...)
			if len(lines) != 2 {
				t.Fatalf("recipe = %q, want the report step and the judge step", lines)
			}
			if got := strings.Join(strings.Fields(lines[0]), " "); got != "go test -coverprofile profile.cov ./..." {
				t.Fatalf("the report step = %q, want the pipeline's shape without -coverpkg", lines[0])
			}
			assertJudgeStep(t, lines[1], tc.base)
		})
	}
}
