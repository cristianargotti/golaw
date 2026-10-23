package assets_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/checks"
	"github.com/cristianargotti/golaw/internal/coverage"
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/proof"
	"github.com/cristianargotti/golaw/internal/runner"
)

// ruleguardModule is the module a rendered ruleguard rules file imports.
const ruleguardModule = "github.com/quasilyte/go-ruleguard/dsl"

// serviceFiles is the service fixture beside every core template init
// renders for the capabilities; the go.mod is the fixture's own, and its
// direct requires reach the inherited baseline as init records them.
func serviceFiles(t *testing.T, capabilities []string) map[string][]byte {
	t.Helper()
	context := assetContext()
	context.Capabilities = capabilities
	service, err := assets.RenderService(golaw.FS, context.Values)
	if err != nil {
		t.Fatal(err)
	}
	context.DirectRequires = directRequires(service)
	core, err := assets.RenderAllContext(golaw.FS, context)
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string][]byte, len(core)+len(service))
	all := slices.Concat(core, service)
	for i := range all {
		files[all[i].Path] = all[i].Data
	}
	return withKitRequirement(t, files, ruleguardModule)
}

// withKitRequirement adds a module the kit's go.mod requires, with its
// go.sum lines, when a rendered rules file imports it: init proposes that
// require and the person applies it, so the tree holds it as an adopter's
// would, in the shape go mod tidy leaves, because the generated code may
// bring requires of its own.
func withKitRequirement(t *testing.T, files map[string][]byte, module string) map[string][]byte {
	t.Helper()
	if _, ok := files["scripts/lint/rules.go"]; !ok {
		return files
	}
	kitModule, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	var require string
	for line := range strings.SplitSeq(string(kitModule), "\n") {
		fields := strings.Fields(strings.TrimPrefix(strings.TrimSpace(line), "require "))
		if len(fields) >= 2 && fields[0] == module {
			require = fields[0] + " " + fields[1]
		}
	}
	if require == "" {
		t.Fatalf("the kit's go.mod requires no %s, which the rendered rules file imports", module)
	}
	kitSum, err := os.ReadFile(filepath.Join("..", "..", "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	var sum strings.Builder
	for line := range strings.SplitSeq(string(kitSum), "\n") {
		if strings.HasPrefix(line, module+" ") {
			if _, err := sum.WriteString(line + "\n"); err != nil {
				t.Fatal(err)
			}
		}
	}
	files["go.mod"] = requireInModule(files["go.mod"], require)
	files["go.sum"] = mergedSum(files["go.sum"], sum.String())
	return files
}

// writeTree writes the files under a fresh directory and answers it.
func writeTree(t *testing.T, files map[string][]byte) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for relative, content := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// TestServiceFixtureScansClean dispatches the production catalog over the
// service beside the core templates: the fixture CI builds is never a tree
// the kit judges red, and a raw error answered by a handler is found there,
// so the scan reaches its Go files.
func TestServiceFixtureScansClean(t *testing.T) {
	t.Parallel()
	registry, err := checks.Registry()
	if err != nil {
		t.Fatal(err)
	}
	raw := "package server\n\ntype responder interface{ JSON(status int, body any) }\n\n" +
		"func answerRaw(r responder) { r.JSON(500, map[string]string{\"message\": \"failed\"}) }\n"
	for _, tc := range []struct {
		name         string
		capabilities []string
		extra        map[string]string
		want         string
	}{
		{"near go", []string{"go"}, nil, ""},
		{"near cli hexagonal image", []string{"cli", "go", "hexagonal", "image"}, nil, ""},
		{"near every capability", model.Capabilities(), nil, ""},
		{"red raw error under api", []string{"api", "go"}, map[string]string{"internal/server/raw.go": raw}, "goast.raw-error-response"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			files := serviceFiles(t, tc.capabilities)
			for path, body := range tc.extra {
				files[path] = []byte(body)
			}
			tree := proof.NewTree(files)
			run, err := engine.Dispatch(t.Context(), registry, model.Input{
				Mode: model.ModeScan, Policy: servicePolicy(t, tc.capabilities), Snapshot: tree, Targets: tree.Files(),
			})
			if err != nil {
				t.Fatal(err)
			}
			requireServiceDecision(t, engine.Decide(run), tc.want)
		})
	}
}

// servicePolicy is the effective policy of the golaw.yml init writes beside
// the templates for the capabilities: the floors and the allowed lint
// exclusions init seeds, which the core defaults do not hold.
func servicePolicy(t *testing.T, capabilities []string) *model.EffectivePolicy {
	t.Helper()
	data, err := bootstrap.Render(bootstrap.Inputs{
		KitVersion: templateValues().KitVersion, Language: model.DefaultLanguage, Capabilities: capabilities,
		Layout: bootstrap.Layout{Main: bootstrap.MainRoot, Decisions: "docs/adr", Go: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	file, err := policy.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := policy.Compile(file, time.Date(2026, time.October, 20, 0, 0, 0, 0, time.UTC), nil)
	if err != nil {
		t.Fatal(err)
	}
	return compiled
}

// requireServiceDecision holds a near tree to no finding and no unmeasured
// check, and a red tree to a finding of the check it names.
func requireServiceDecision(t *testing.T, decision engine.Decision, want string) {
	t.Helper()
	if want != "" {
		if !slices.ContainsFunc(decision.Findings, func(f model.Finding) bool { return f.Check == want }) {
			t.Fatalf("no finding of %s in %+v", want, decision.Findings)
		}
		return
	}
	for _, finding := range decision.Findings {
		t.Errorf("%s:%d %s %s", finding.Path, finding.Line, finding.Check, finding.Message)
	}
	for _, left := range decision.Unmeasured {
		t.Errorf("unmeasured: %+v", left)
	}
}

// TestServiceFixtureBuildsTestsAndMeetsTheFloors vets and tests the
// rendered service with its coverage profile and holds it to the floors of
// section 3: 80 percent total, per package and per file over 20 statements.
func TestServiceFixtureBuildsTestsAndMeetsTheFloors(t *testing.T) {
	t.Parallel()
	service, err := assets.RenderService(golaw.FS, templateValues())
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string][]byte, len(service))
	for i := range service {
		files[service[i].Path] = service[i].Data
	}
	root := writeTree(t, files)
	env := append((&exec.Cmd{}).Environ(), "GOTOOLCHAIN=local", "GOFLAGS=", "GOWORK=off")
	profile := filepath.Join(root, "coverage.out")
	for _, c := range []runner.Command{
		{Name: "go", Args: []string{"vet", "./..."}, Dir: root, Env: env},
		{Name: "go", Args: []string{"test", "-count=1", "-coverpkg=./...", "-coverprofile=" + profile, "./..."}, Dir: root, Env: env},
	} {
		out, runErr := runner.Run(t.Context(), c)
		if runErr != nil || out.Code != 0 {
			t.Fatalf("%s %v = %d: %s%s: %v", c.Name, c.Args, out.Code, out.Stdout, out.Stderr, runErr)
		}
	}
	input, err := os.Open(profile)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	report, err := coverage.Evaluate(input, root, coverage.DefaultFloors())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Failures) != 0 {
		t.Fatalf("coverage floors: %v", report.Lines())
	}
}

// TestServiceFixtureLintsClean runs the pinned golangci-lint with the
// strict asset rendered for the hexagonal capability and go mod tidy -diff
// over the service, as make lint and the deadcode target do. The modules
// the generated API code imports come through the proxy the environment
// names, as the other targets of make ci read theirs; a warm module cache
// serves them without the network.
func TestServiceFixtureLintsClean(t *testing.T) {
	t.Parallel()
	tool, ok := runner.LookupTool("golangci-lint")
	if !ok {
		t.Fatal("golangci-lint is not a pinned tool")
	}
	env := append((&exec.Cmd{}).Environ(), "GOTOOLCHAIN=local", "GOFLAGS=-mod=mod", "GOWORK=off")
	linter, err := runner.Verify(t.Context(), tool, env)
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("CI installs golangci-lint %s before the tests: %v", tool.Version, err)
		}
		t.Skipf("golangci-lint %s is not on PATH, which make ci installs: %v", tool.Version, err)
	}
	root := writeTree(t, serviceFiles(t, []string{"go", "hexagonal"}))
	for _, c := range []runner.Command{
		{Name: "go", Args: []string{"mod", "tidy", "-diff"}, Dir: root, Env: env},
		{Name: linter, Args: []string{"run", "--allow-parallel-runners", "./..."}, Dir: root, Env: env},
	} {
		out, err := runner.Run(t.Context(), c)
		if err != nil || out.Code != 0 {
			t.Errorf("%s %v = %d: %s%s: %v", c.Name, c.Args, out.Code, out.Stdout, out.Stderr, err)
		}
	}
}
