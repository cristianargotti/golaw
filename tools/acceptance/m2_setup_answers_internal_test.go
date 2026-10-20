package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/setup"
)

// m2Setup stands in for golaw setup: every pinned tool written into GOBIN
// when absent, the three hooks lefthook writes and the receipt last; a run
// that changes nothing prints nothing, and alter rewrites the pre-commit
// hook of an armed repository and prints nothing.
func m2Setup(ctx context.Context, t *testing.T, s *suite, c command, alter bool) result {
	t.Helper()
	path := m2ReceiptPath(t, s, c)
	before, readErr := os.ReadFile(path)
	hooks := m2HooksDir(ctx, t, s, c)
	if alter && readErr == nil {
		m2Write(t, filepath.Join(hooks, "pre-commit"), m2HookBody("pre-commit")+"# altered\n")
		return result{}
	}
	tools, installed := m2InstallTools(t, s, c)
	artifacts, wrote := m2WriteHooks(t, hooks)
	root, identity, err := install.ProjectIdentity(c.dir)
	if err != nil {
		t.Fatal(err)
	}
	receipt := setup.Receipt{
		Schema: setup.ReceiptSchema, Identity: identity, Root: root, Version: m2Version,
		Tools: tools, Hooks: artifacts, HooksPath: hooks, Golaw: s.binary, ArmedAt: m2Moment(),
	}
	if writeErr := setup.WriteReceipt(path, receipt); writeErr != nil {
		t.Fatal(writeErr)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if readErr == nil && !installed && !wrote && bytes.Equal(before, after) {
		return result{}
	}
	return result{stdout: "golaw setup: armed " + root + "\n"}
}

// m2InstallTools writes every pinned tool missing from GOBIN, or from the
// shared tool directory when the command names no GOBIN, and reports
// whether it wrote one.
func m2InstallTools(t *testing.T, s *suite, c command) ([]setup.ToolRecord, bool) {
	t.Helper()
	bin := environmentValue(c.env, "GOBIN")
	if bin == "" {
		bin = s.m2.Bin
	}
	tools := runner.Toolchain()
	records := make([]setup.ToolRecord, 0, len(tools))
	installed := false
	for _, tool := range tools {
		path := filepath.Join(bin, executable(tool.Name))
		if _, err := os.Lstat(path); err != nil {
			installed = true
			m2Write(t, path, "simulated "+tool.Name+"\n")
		}
		records = append(records, setup.ToolRecord{Name: tool.Name, Version: tool.Version, Path: path})
	}
	return records, installed
}

// m2WriteHooks writes the three hooks lefthook writes into the hooks
// directory and reports whether one changed.
func m2WriteHooks(t *testing.T, dir string) ([]install.Artifact, bool) {
	t.Helper()
	rungs := []string{"pre-commit", "commit-msg", "pre-push"}
	artifacts := make([]install.Artifact, 0, len(rungs))
	wrote := false
	for _, rung := range rungs {
		path, body := filepath.Join(dir, rung), m2HookBody(rung)
		if current, err := os.ReadFile(path); err != nil || string(current) != body {
			wrote = true
			m2Write(t, path, body)
		}
		artifacts = append(artifacts, install.Artifact{Path: path, SHA256: install.Digest([]byte(body))})
	}
	return artifacts, wrote
}

// m2HookBody is the hook file lefthook writes for a rung.
func m2HookBody(rung string) string {
	return "#!/bin/sh\nexec lefthook run " + rung + " \"$@\"\n"
}

// m2HooksDir is the hooks directory the real Git routes the repository of
// a command to.
func m2HooksDir(ctx context.Context, t *testing.T, s *suite, c command) string {
	t.Helper()
	got := runCommand(ctx, command{
		program: s.git, args: []string{"rev-parse", "--path-format=absolute", "--git-path", "hooks"},
		dir: c.dir, env: c.env,
	})
	if err := success(got); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(got.stdout)
}

// m2Armed reports whether the repository of a command holds a pre-commit
// hook, so a commit there runs through the hooks.
func m2Armed(ctx context.Context, t *testing.T, s *suite, c command) bool {
	t.Helper()
	_, err := os.Lstat(filepath.Join(m2HooksDir(ctx, t, s, c), "pre-commit"))
	return err == nil
}

// m2ReceiptPath is where setup keeps the receipt of the repository a
// command runs in, under the state directory its environment selects.
func m2ReceiptPath(t *testing.T, s *suite, c command) string {
	t.Helper()
	_, identity, err := install.ProjectIdentity(c.dir)
	if err != nil {
		t.Fatal(err)
	}
	return setup.ReceiptPath(m2Dirs(t, s, c.env).State, identity)
}

// m2Uninstall stands in for golaw setup --uninstall: the receipt and the
// hooks it names go and the tools stay; corrupt also removes post-merge,
// the unrelated hook beside them.
func m2Uninstall(t *testing.T, s *suite, c command, corrupt bool) result {
	t.Helper()
	path := m2ReceiptPath(t, s, c)
	receipt, err := setup.ReadReceipt(path)
	if err != nil {
		return result{code: 2, stderr: "golaw setup: " + err.Error() + "\n"}
	}
	paths := make([]string, 0, len(receipt.Hooks)+2)
	paths = append(paths, path)
	for _, hook := range receipt.Hooks {
		paths = append(paths, hook.Path)
	}
	if corrupt {
		paths = append(paths, filepath.Join(receipt.HooksPath, "post-merge"))
	}
	for _, each := range paths {
		if removeErr := os.Remove(each); removeErr != nil {
			t.Fatal(removeErr)
		}
	}
	return result{stdout: "golaw setup: disarmed " + receipt.Root + "\n"}
}

// m2Doctor answers doctor --json from the receipt setup left: a line per
// tool with its pin, the receipt path and the hooks directory.
func m2Doctor(t *testing.T, s *suite, c command) result {
	t.Helper()
	path := m2ReceiptPath(t, s, c)
	receipt, err := setup.ReadReceipt(path)
	if err != nil {
		return result{code: 1, stderr: "golaw doctor: " + err.Error() + "\n"}
	}
	lines := []doctor.Line{
		{ID: "doctor.setup", Key: "receipt", State: "ok", Code: "armed", Detail: path},
		{ID: "doctor.setup", Key: "hooks", State: "ok", Code: "routed", Detail: receipt.HooksPath},
	}
	for _, tool := range receipt.Tools {
		lines = append(lines, doctor.Line{
			ID: "doctor.tools", Key: tool.Name, State: "ok", Code: "pinned",
			Detail: tool.Name + " " + tool.Version + " at " + tool.Path,
		})
	}
	data, err := json.Marshal(doctor.Report{Schema: 1, Lines: lines})
	if err != nil {
		t.Fatal(err)
	}
	return result{stdout: string(data) + "\n"}
}

// m2Update stands in for init --update: the preview names each template a
// local edit keeps and writes nothing; the apply writes every template the
// edit spares, pins the candidate and enrolls; corrupt replaces the edit.
func m2Update(t *testing.T, s *suite, c command, corrupt bool) result {
	t.Helper()
	apply := slices.Contains(c.args, flagApply)
	lines := []string{"golaw init --update: " + m2Version}
	files := m2Render(t, c.dir)
	for i := range files {
		path := filepath.Join(c.dir, filepath.FromSlash(files[i].Path))
		current, err := os.ReadFile(path)
		if err == nil && !bytes.Equal(current, files[i].Data) && !corrupt {
			lines = append(lines, files[i].Path+": local edit kept")
			continue
		}
		if apply {
			m2Write(t, path, string(files[i].Data))
		}
	}
	if apply {
		m2Pin(t, c.dir, m2Version)
		m2Enroll(t, s, c.dir, c.env)
	}
	return result{stdout: strings.Join(lines, "\n") + "\n"}
}

// m2Pin sets the kit version of the golaw.yml a checkout holds.
func m2Pin(t *testing.T, dir, version string) {
	t.Helper()
	path := filepath.Join(dir, policyFile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	pin := regexp.MustCompile(`(?m)^kit: \{ version: \S+ \}$`)
	m2Write(t, path, pin.ReplaceAllLiteralString(string(data), "kit: { version: "+version+" }"))
}

// m2ProbeResult answers the version probe of a pinned tool with its pin,
// or with another version when wrong is set.
func m2ProbeResult(c command, wrong bool) result {
	name := strings.TrimSuffix(filepath.Base(c.program), ".exe")
	tool, _ := runner.LookupTool(name)
	version := tool.Version
	if wrong {
		version += "-other"
	}
	return result{stdout: name + " version " + version + "\n"}
}
