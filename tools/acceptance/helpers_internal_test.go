package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func checkedWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := writeFile(path, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
}

func testRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, err := moduleRoot(cwd)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func simulatedExecutor(t *testing.T, root string) executor {
	t.Helper()
	return func(_ context.Context, c command) result {
		if slices.Contains(c.args, "build") {
			return simulatedBuild(t, c)
		}
		if len(c.args) == 0 {
			return result{err: ErrAcceptance}
		}
		return simulatedProgram(t, root, c)
	}
}

// simulatedBuild writes the executable a build command would produce.
func simulatedBuild(t *testing.T, c command) result {
	t.Helper()
	index := slices.Index(c.args, "-o")
	if index < 0 || index+1 >= len(c.args) {
		return result{err: ErrAcceptance}
	}
	checkedWrite(t, c.args[index+1], "simulated executable")
	return result{}
}

// simulatedProgram answers the init, install and version commands.
func simulatedProgram(t *testing.T, root string, c command) result {
	t.Helper()
	switch c.args[0] {
	case "init":
		if filepath.Base(c.program) == executable(binaryName) {
			checkedWrite(t, filepath.Join(c.dir, policyFile), "file_lines: 300\n")
		}
	case "agent":
		body, err := readFile(filepath.Join(root, "agents", agentClaude, "hooks", hooksFileName))
		if err != nil {
			return result{err: err}
		}
		home := environmentValue(c.env, "HOME")
		checkedWrite(t, simulatedArtifacts(home)[0], string(body))
		checkedWrite(t, filepath.Join(home, "state", "receipt.json"), simulatedReceipt(t, home, body))
	case "--version":
		return result{stdout: "2.1.299 (Claude Code)\n"}
	}
	return result{}
}

// simulatedArtifacts are the artifacts the simulated installer lists in its
// receipt: the plugin hook file it writes and the settings file.
func simulatedArtifacts(home string) []string {
	return []string{
		filepath.Join(home, ".claude", "plugins", binaryName, "hooks", hooksFileName),
		filepath.Join(home, ".claude", "settings.json"),
	}
}

// simulatedReceipt is the receipt of the simulated installation, shaped
// like the one the installer writes: its artifacts and the hook definition
// the plugin ships.
func simulatedReceipt(t *testing.T, home string, definition []byte) string {
	t.Helper()
	type artifact struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	}
	artifacts := make([]artifact, 0, len(simulatedArtifacts(home)))
	for _, path := range simulatedArtifacts(home) {
		artifacts = append(artifacts, artifact{Path: path, SHA256: "simulated"})
	}
	body, err := json.Marshal(struct {
		Schema         int             `json:"schema"`
		Artifacts      []artifact      `json:"artifacts"`
		HookDefinition json.RawMessage `json:"hook_definition"`
	}{1, artifacts, definition})
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func testSuite(t *testing.T) *suite {
	t.Helper()
	root := testRoot(t)
	s := newSuite(root, t.TempDir(), simulatedExecutor(t, root),
		func(name string) (string, error) { return name, nil })
	if err := s.prepare(t.Context()); err != nil {
		t.Fatal(err)
	}
	return s
}

func assertPass(t *testing.T, item row) {
	t.Helper()
	if item.status != statusPass {
		t.Fatalf("%+v", item)
	}
}
