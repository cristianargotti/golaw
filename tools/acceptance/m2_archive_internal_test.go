package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestM2ProxyUsesTrackedCurrentBytesAndSortedEntries(t *testing.T) {
	t.Parallel()
	root, proxy := t.TempDir(), filepath.Join(t.TempDir(), "proxy área #1")
	mod := "module github.com/cristianargotti/golaw\n\ngo 1.27\n"
	checkedWrite(t, filepath.Join(root, "go.mod"), mod)
	checkedWrite(t, filepath.Join(root, "z.go"), "package current\n")
	checkedWrite(t, filepath.Join(root, "a.go"), "package first\n")
	address, err := writeM2Proxy(root, proxy, "0.2.0", []string{"z.go", "go.mod", "a.go"})
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(address)
	if err != nil || u.Scheme != "file" || u.Fragment != "" || u.RawQuery != "" {
		t.Fatalf("proxy URL %q: %v", address, err)
	}
	if !strings.HasSuffix(u.Path, "/proxy área #1") {
		t.Fatalf("proxy URL lost its native path: %q", address)
	}
	dir := filepath.Join(proxy, "github.com", "cristianargotti", "golaw", "@v")
	reader, err := zip.OpenReader(filepath.Join(dir, "v0.2.0.zip"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	names := make([]string, 0, len(reader.File))
	for _, entry := range reader.File {
		names = append(names, entry.Name)
	}
	prefix := "github.com/cristianargotti/golaw@v0.2.0/"
	if !slices.Equal(names, []string{prefix + "a.go", prefix + "go.mod", prefix + "z.go"}) {
		t.Fatalf("archive order = %q", names)
	}
	info, err := os.ReadFile(filepath.Join(dir, "v0.2.0.info"))
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]string
	if err := json.Unmarshal(info, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["Time"] != "2026-10-19T00:00:00Z" || fields["Version"] != "v0.2.0" {
		t.Fatalf("proxy info = %v", fields)
	}
}

func TestM2ProxyRejectsUnsafeAndRepeatedPathsWithoutWrites(t *testing.T) {
	t.Parallel()
	for _, paths := range [][]string{
		nil,
		{"a.go"},
		{"go.mod", "go.mod"},
		{"go.mod", ""},
		{"go.mod", "."},
		{"go.mod", "/absolute.go"},
		{"go.mod", "C:/absolute.go"},
		{"go.mod", "a/../a.go"},
		{"go.mod", "./a.go"},
		{"go.mod", "a//b.go"},
		{"go.mod", "a\\b.go"},
		{"go.mod", ".git/config"},
		{"go.mod", "a/.Git/config"},
		{"go.mod", "folder"},
		{"go.mod", "absent.go"},
	} {
		root, proxy := t.TempDir(), filepath.Join(t.TempDir(), "unwritten")
		checkedWrite(t, filepath.Join(root, "go.mod"), "module example.invalid/test\n")
		checkedWrite(t, filepath.Join(root, "a.go"), "package test\n")
		if err := os.Mkdir(filepath.Join(root, "folder"), dirMode); err != nil {
			t.Fatal(err)
		}
		if _, err := writeM2Proxy(root, proxy, "0.2.0", paths); err == nil {
			t.Fatalf("unsafe paths accepted: %q", paths)
		}
		if _, err := os.Lstat(proxy); !os.IsNotExist(err) {
			t.Fatalf("invalid inputs wrote the proxy: %q %v", paths, err)
		}
	}
}

func TestM2ProxyRejectsSymlinkFilesAndParents(t *testing.T) {
	t.Parallel()
	for _, parent := range []bool{false, true} {
		root, outside := t.TempDir(), t.TempDir()
		checkedWrite(t, filepath.Join(root, "go.mod"), "module example.invalid/test\n")
		checkedWrite(t, filepath.Join(outside, "safe.go"), "package outside\n")
		target, name := filepath.Join(outside, "safe.go"), "link.go"
		if parent {
			target, name = outside, "linked"
		}
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Skipf("host cannot create symbolic links: %v", err)
		}
		if parent {
			name += "/safe.go"
		}
		if _, err := writeM2Proxy(root, t.TempDir(), "0.2.0", []string{"go.mod", name}); err == nil {
			t.Fatalf("symbolic link accepted: %s", name)
		}
	}
}

func TestM2ProxyReportsDestinationFailures(t *testing.T) {
	t.Parallel()
	for _, blocked := range []string{"proxy", "list", "v0.2.0.mod", "v0.2.0.info", "v0.2.0.zip"} {
		root, proxy := t.TempDir(), filepath.Join(t.TempDir(), "proxy")
		checkedWrite(t, filepath.Join(root, "go.mod"), "module example.invalid/test\n")
		if blocked == "proxy" {
			checkedWrite(t, proxy, "file blocks proxy directory")
		} else {
			path := filepath.Join(proxy, "github.com", "cristianargotti", "golaw", "@v", blocked)
			if err := os.MkdirAll(path, dirMode); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := writeM2Proxy(root, proxy, "0.2.0", []string{"go.mod"}); err == nil {
			t.Fatalf("destination failure accepted: %s", blocked)
		}
	}
}

func TestM2ProxyReportsSourceReadFailure(t *testing.T) {
	t.Parallel()
	root, proxy := t.TempDir(), filepath.Join(t.TempDir(), "unwritten")
	source := filepath.Join(root, "go.mod")
	checkedWrite(t, source, "module example.invalid/test\n")
	if err := os.Chmod(source, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(source, fileMode); err != nil {
			t.Error(err)
		}
	})
	if _, err := os.ReadFile(source); err == nil {
		t.Skip("host privileges allow reading a file with no permissions")
	}
	if _, err := writeM2Proxy(root, proxy, "0.2.0", []string{"go.mod"}); err == nil {
		t.Fatal("unreadable source accepted")
	}
	if _, err := os.Lstat(proxy); !os.IsNotExist(err) {
		t.Fatalf("unreadable source wrote the proxy: %v", err)
	}
}

func TestM2ArchiveReportsWriterErrors(t *testing.T) {
	t.Parallel()
	files := []m2ArchiveFile{{name: "go.mod", data: []byte("module example.invalid/test\n")}}
	var output bytes.Buffer
	if err := writeM2Archive(&output, "0.2.0", files); err != nil {
		t.Fatal(err)
	}
	if _, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len())); err != nil {
		t.Fatal(err)
	}
	if err := writeM2Archive(failingWriter{}, "0.2.0", files); err == nil {
		t.Fatal("archive writer failure passed")
	}
	if err := writeM2Archive(&output, "0.2.0", []m2ArchiveFile{{name: ""}}); err == nil {
		t.Fatal("empty archive entry accepted")
	}
	if err := writeM2Archive(&output, "0.2.0", []m2ArchiveFile{{name: strings.Repeat("a", 1<<16)}}); err == nil {
		t.Fatal("oversized archive entry accepted")
	}
}

func TestM2ProxyCurrentBytesAreNotSourceSnapshots(t *testing.T) {
	t.Parallel()
	root, proxy := t.TempDir(), t.TempDir()
	checkedWrite(t, filepath.Join(root, "go.mod"), "module example.invalid/old\n")
	checkedWrite(t, filepath.Join(root, "go.mod"), "module example.invalid/current\n")
	if _, err := writeM2Proxy(root, proxy, "0.2.0", []string{"go.mod"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(proxy, "github.com", "cristianargotti", "golaw", "@v", "v0.2.0.zip")
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	file, err := reader.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(file)
	closeErr := file.Close()
	if err != nil || closeErr != nil || string(body) != "module example.invalid/current\n" {
		t.Fatalf("current bytes = %q, read = %v, close = %v", body, err, closeErr)
	}
}
