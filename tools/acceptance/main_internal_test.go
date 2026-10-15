package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunFailsWithoutInstallationRows(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	if run(&out, runCommand, exec.LookPath) != 1 ||
		!strings.Contains(out.String(), "\tFAIL\tinstallation\t") {
		t.Fatal(out.String())
	}
}

func TestExecutableWrite(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "bin", executable("golaw"))
	if err := writeFile(path, []byte("simulated executable"), execMode); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("%v, %v", info, err)
	}
}
