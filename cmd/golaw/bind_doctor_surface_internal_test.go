package main

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/toolchain"
	"github.com/cristianargotti/golaw/internal/toolchaintest"
)

func surfaceLines(lines []doctor.Line) map[string]doctor.Line {
	keyed := make(map[string]doctor.Line)
	for _, line := range lines {
		keyed[line.ID+" "+line.Key] = line
	}
	return keyed
}

func TestSurfaceDoctorNamesEachToolOfTheLanguagesHeld(t *testing.T) {
	t.Parallel()
	b, tools, _ := surfaceBinding(t)
	ruff, _ := toolchain.Lookup("ruff")
	toolchaintest.Install(t, tools, ruff, runtime.GOOS, runtime.GOARCH, runnertest.Behavior{})
	root := repository(t, true)
	writeFile(t, filepath.Join(root, "scripts", "check.py"), "import os\n")
	writeFile(t, filepath.Join(root, "Dockerfile"), "FROM scratch\n")
	got := surfaceLines(b.surfaceDoctor(t.Context(), root, now()))
	if line := got["doctor.surface-tools ruff"]; line.Code != "surface-tool-available" || !strings.Contains(line.Detail, "sha256 verified") {
		t.Errorf("ruff = %+v", line)
	}
	for _, missing := range []string{"hadolint", "checkov", "pyright"} {
		if line := got["doctor.surface-tools "+missing]; line.Code != "surface-tool-missing" {
			t.Errorf("%s = %+v, want the missing line", missing, line)
		}
	}
	for _, rt := range []string{"node", "python"} {
		if line := got["doctor.surface-runtimes "+rt]; line.Code != "surface-runtime-missing" {
			t.Errorf("%s = %+v, want the missing runtime", rt, line)
		}
	}
	if _, ok := got["doctor.surface-tools eslint"]; ok {
		t.Error("a line for a language the repository does not hold")
	}
	if line := got["doctor.surface-tools checkov"]; !strings.Contains(line.Detail, "dockerfile is not linted") {
		t.Errorf("checkov names %q, want only the held language", line.Detail)
	}
}
