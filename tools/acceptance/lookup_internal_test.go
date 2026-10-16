package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestExecutableLookup(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	binary := filepath.Join(root, executable(binaryName))
	checkedWrite(t, binary, "binary")
	env := []string{pathAssignment + string(os.PathListSeparator) + root}
	got, err := resolveExecutable(binaryName, env)
	if err != nil || got != binary {
		t.Fatalf("%s %v", got, err)
	}
	if _, err := resolveExecutable("absent", env); err == nil {
		t.Fatal("absent executable passed")
	}
	if err := os.Mkdir(filepath.Join(root, executable("directory")), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveExecutable("directory", env); err == nil {
		t.Fatal("directory passed")
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(binary, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := resolveExecutable(binaryName, env); err == nil {
			t.Fatal("non executable file passed")
		}
	}
}
