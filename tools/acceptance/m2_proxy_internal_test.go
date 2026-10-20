package main

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestM2ProxyLayout(t *testing.T) {
	t.Parallel()
	root, proxy := t.TempDir(), t.TempDir()
	files := map[string]string{
		"go.mod":            "module github.com/cristianargotti/golaw\n\ngo 1.27\n",
		"cmd/golaw/main.go": "package main\n\nfunc main() {}\n",
	}
	for path, body := range files {
		checkedWrite(t, filepath.Join(root, filepath.FromSlash(path)), body)
	}
	checkedWrite(t, filepath.Join(root, "untracked.txt"), "not archived\n")
	paths := []string{"go.mod", "cmd/golaw/main.go"}
	if _, err := writeM2Proxy(root, proxy, "0.2.0", paths); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(proxy, "github.com", "cristianargotti", "golaw", "@v")
	for name, want := range map[string]string{
		"list":       "v0.2.0\n",
		"v0.2.0.mod": files["go.mod"],
	} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(got) != want {
			t.Fatalf("%s: body=%q error=%v", name, got, err)
		}
	}
	m2ProxyInfo(t, filepath.Join(dir, "v0.2.0.info"))
	want := []string{
		"github.com/cristianargotti/golaw@v0.2.0/cmd/golaw/main.go",
		"github.com/cristianargotti/golaw@v0.2.0/go.mod",
	}
	if names := m2ArchiveNames(t, filepath.Join(dir, "v0.2.0.zip"), files); !slices.Equal(names, want) {
		t.Fatalf("zip entries = %q", names)
	}
}

// m2ProxyInfo requires the info file of the proxy to name the version and
// a time.
func m2ProxyInfo(t *testing.T, path string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var info struct {
		Version string
		Time    string
	}
	if decodeErr := json.Unmarshal(body, &info); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if info.Version != "v0.2.0" || info.Time == "" {
		t.Fatalf("invalid proxy info: %+v", info)
	}
}

// m2ArchiveNames reads the module zip of the proxy, requires every entry to
// hold the bytes of its tracked file, and answers the sorted entry names.
func m2ArchiveNames(t *testing.T, path string, files map[string]string) []string {
	t.Helper()
	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := archive.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	names := make([]string, 0, len(archive.File))
	for _, entry := range archive.File {
		names = append(names, entry.Name)
		reader, openErr := entry.Open()
		if openErr != nil {
			t.Fatal(openErr)
		}
		got, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil || closeErr != nil {
			t.Fatalf("zip read: %v %v", readErr, closeErr)
		}
		rel := entry.Name[len("github.com/cristianargotti/golaw@v0.2.0/"):]
		if string(got) != files[rel] {
			t.Fatalf("wrong archived bytes for %s", rel)
		}
	}
	slices.Sort(names)
	return names
}

func TestM2ProxyRejectsInvalidInputs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		version string
		paths   []string
	}{
		{"empty version", "", []string{"go.mod"}},
		{"moving version", "latest", []string{"go.mod"}},
		{"short version", "0.2", []string{"go.mod"}},
		{"version traversal", "../0.2.0", []string{"go.mod"}},
		{"path traversal", "0.2.0", []string{"../go.mod"}},
		{"missing source", "0.2.0", []string{"missing.go"}},
	}
	for _, tt := range tests {
		root := t.TempDir()
		checkedWrite(t, filepath.Join(root, "go.mod"),
			"module github.com/cristianargotti/golaw\n\ngo 1.27\n")
		_, err := writeM2Proxy(root, t.TempDir(), tt.version, tt.paths)
		if err == nil {
			t.Fatalf("%s: invalid proxy input accepted", tt.name)
		}
	}
}
