package goldentest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/source"
)

// configMode is the mode of the harness's own lint configuration.
const configMode = 0o600

// The names the lint and format steps resolve.
const (
	lintTool   = "golangci-lint"
	lintAsset  = ".golangci.yml"
	lintConfig = "golangci.yml"
	// adoptedConfig is the asset as the adopter's make lint reads it.
	adoptedConfig = "golangci-adopted.yml"
	gitRootMode   = "gitroot"
	rulesFile     = "scripts/lint/rules.go"
	// disableGenerated judges a file whatever its generated header says.
	disableGenerated = "disable"
)

// goCommand runs the go command in the module; a nil environment inherits
// the test's own, and go test puts the running toolchain first on its PATH.
func goCommand(ctx context.Context, dir string, args ...string) (runner.Output, error) {
	out, err := runner.Run(ctx, runner.Command{Name: "go", Args: args, Dir: dir})
	if err != nil {
		return out, fmt.Errorf("go %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

// pinned answers the path of a pinned tool at its pin, or why a step that
// needs it is skipped.
func pinned(ctx context.Context, name string) (string, string) {
	tool, ok := runner.LookupTool(name)
	if !ok {
		return "", name + " is not a pinned tool"
	}
	path, err := runner.Verify(ctx, tool, nil)
	if err != nil {
		return "", fmt.Sprintf("%s %s not found: run golaw setup --gen (%v)", tool.Name, tool.Version, err)
	}
	return path, ""
}

// dslCached answers whether the ruleguard dsl the lint asset loads is in
// the module cache, since the scratch module may not reach the network.
func dslCached(ctx context.Context, root string) (bool, string) {
	missing := dslModule + " " + dslVersion + " is not in the module cache: run go mod download in the kit"
	out, err := goCommand(ctx, root, "env", "GOMODCACHE")
	if err != nil || out.Code != 0 {
		return false, missing
	}
	cache := strings.TrimSpace(string(out.Stdout))
	dir := filepath.Join(cache, filepath.FromSlash(dslModule)+"@"+dslVersion)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return false, missing
	}
	return true, ""
}

// resolveLint answers the pinned golangci-lint, or why the lint steps skip:
// the tool at another version, or the ruleguard dsl out of the cache.
func resolveLint(ctx context.Context, root string) (string, string) {
	path, reason := pinned(ctx, lintTool)
	if reason == "" {
		_, reason = dslCached(ctx, root)
	}
	return path, reason
}

// lintConfig writes the rendered lint asset with the harness's changes
// outside the module: the asset's generated: lax would skip every golaw
// regenerable file the pipeline analyses, and the third party outputs stay
// excluded as the pipeline excludes them. asAdopted keeps the asset's
// exclusions as they are and changes only how the run finds its files.
func (h *harness) lintConfig(asAdopted bool) (string, error) {
	data, err := os.ReadFile(source.OSPath(h.repo.Root, lintAsset))
	if err != nil {
		return "", fmt.Errorf(readFailed, lintAsset, err)
	}
	var config map[string]any
	if decodeErr := yaml.Unmarshal(data, &config); decodeErr != nil {
		return "", fmt.Errorf("decode %s: %w", lintAsset, decodeErr)
	}
	linters := section(config, "linters")
	if !asAdopted {
		judgeGenerated(config, linters)
	}
	section(config, "run")["relative-path-mode"] = gitRootMode
	// ruleguard reads its rules as a comma separated list, and a temporary
	// directory named after a subtest can hold a comma, so the rules are
	// named from the module root the run starts in.
	ruleguard := section(section(section(section(linters, "settings"), "gocritic"), "settings"), "ruleguard")
	ruleguard["rules"] = rulesFile
	encoded, err := yaml.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("encode the lint configuration: %w", err)
	}
	name := lintConfig
	if asAdopted {
		name = adoptedConfig
	}
	path := filepath.Join(h.work, name)
	if err := os.WriteFile(path, encoded, configMode); err != nil {
		return "", fmt.Errorf("write the lint configuration: %w", err)
	}
	return path, nil
}

// judgeGenerated makes the linters and the formatters judge a generated
// file, the golaw ones the pipeline analyses, while the third party output
// directories stay excluded.
func judgeGenerated(config, linters map[string]any) {
	exclusions := section(linters, "exclusions")
	exclusions["generated"] = disableGenerated
	// The formatters skip a generated file by default as well, which would
	// leave the format step judging nothing.
	section(section(config, "formatters"), "exclusions")["generated"] = disableGenerated
	var paths []any
	if listed, ok := exclusions["paths"].([]any); ok {
		paths = listed
	}
	exclusions["paths"] = append(paths, "(^|/)gen/", "(^|/)sqlcgen/")
}

// section answers the mapping under key, creating it when absent.
func section(parent map[string]any, key string) map[string]any {
	child, ok := parent[key].(map[string]any)
	if !ok {
		child = map[string]any{}
		parent[key] = child
	}
	return child
}
