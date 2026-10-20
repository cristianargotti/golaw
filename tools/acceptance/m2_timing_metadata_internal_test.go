package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/runner"
)

func TestM2TimingMetadataNamesItsMeasuredInputs(t *testing.T) {
	t.Parallel()
	for _, live := range []bool{false, true} {
		s := m2MetadataSuite(t)
		s.m2.Live = live
		probes := m2TrackMetadataProbes(t, s)
		got, err := s.timingMetadataM2(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		m2RequireTimingMetadata(t, s, got, live)
		for _, tool := range runner.Toolchain() {
			want := 1
			if live && tool.Name == m2Scanner {
				want++
			}
			if got[tool.Name] != tool.Version || probes[tool.Name] != want {
				t.Fatalf("%s metadata=%q probes=%d", tool.Name, got[tool.Name], probes[tool.Name])
			}
		}
	}
}

func m2TrackMetadataProbes(t *testing.T, s *suite) map[string]int {
	t.Helper()
	fixture := s.execute
	probes := make(map[string]int)
	s.execute = func(ctx context.Context, c command) result {
		if tool, found := runner.LookupTool(strings.TrimSuffix(filepath.Base(c.program), ".exe")); found {
			if !slices.Equal(c.args, tool.Probe) || c.program != filepath.Join(s.m2.Bin, executable(tool.Name)) {
				t.Fatalf("metadata did not use the verified fixed probe: %+v", c)
			}
			probes[tool.Name]++
		}
		return fixture(ctx, c)
	}
	return probes
}

func m2MetadataSuite(t *testing.T) *suite {
	t.Helper()
	s := m2TestSuite(t)
	s.root = t.TempDir()
	m2Write(t, filepath.Join(s.root, moduleFile), "module example.test/metadata\n\ngo 1.27\n")
	m2Write(t, filepath.Join(s.root, "internal", "source", "doc.go"), "package source\n")
	script := m2TimingScript(t, s, "answer")
	s.execute = func(ctx context.Context, c command) result {
		if c.program == s.git && slices.Contains(c.args, m2RevParse) {
			return result{stdout: strings.Repeat("a", 40) + "\n"}
		}
		if c.program == s.git && slices.Equal(c.args, []string{"--version"}) {
			return result{stdout: "git version 2.50.1.windows.1 (source " + s.home + ")\n"}
		}
		return script.Execute(ctx, c)
	}
	return s
}

func m2RequireTimingMetadata(t *testing.T, s *suite, got map[string]string, live bool) {
	t.Helper()
	for _, key := range []string{"revision", "tree", "go", "git", "database", "database_source", "samples", "caches"} {
		if got[key] == "" {
			t.Fatalf("metadata lacks %s", key)
		}
	}
	if got["revision"] != strings.Repeat("a", 40) || len(got["tree"]) != 64 ||
		got["go"] != "go1.27.0" || got["git"] != "2.50.1.windows.1" || got["samples"] != "30" {
		t.Fatalf("metadata misstates source or versions: %v", got)
	}
	database, source := "fixture", "tools/acceptance/testdata/m2/empty-db"
	if live {
		database, source = "live", "https://vuln.go.dev"
	}
	if got["database"] != database || got["database_source"] != source {
		t.Fatalf("database metadata differs: %v", got)
	}
	for _, name := range []string{"GOCACHE", "GOMODCACHE", "GOLANGCI_LINT_CACHE", "cold", "warm", "priming"} {
		if !strings.Contains(got["caches"], name) {
			t.Fatalf("cache metadata omits %s: %s", name, got["caches"])
		}
	}
	for key, value := range got {
		if strings.Contains(value, s.home) || strings.Contains(value, s.base) || strings.Contains(value, s.root) ||
			strings.Contains(strings.ToLower(key), "timestamp") {
			t.Fatalf("metadata leaked a machine path or invented a timestamp: %s=%s", key, value)
		}
	}
}

func TestM2TimingMetadataTracksCurrentSourceBytes(t *testing.T) {
	t.Parallel()
	s := m2MetadataSuite(t)
	before, err := s.timingMetadataM2(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	m2Write(t, filepath.Join(s.root, "internal", "source", "doc.go"), "package source\n\nconst Current = true\n")
	after, err := s.timingMetadataM2(t.Context())
	if err != nil || before["revision"] != after["revision"] || before["tree"] == after["tree"] {
		t.Fatalf("current source edit was not represented: before=%v after=%v error=%v", before, after, err)
	}
}

func TestM2TimingTreeIncludesNamesAndContentWithoutReordering(t *testing.T) {
	t.Parallel()
	first := []m2ArchiveFile{{name: "b", data: []byte("second")}, {name: "a", data: []byte("first")}}
	ordered := []m2ArchiveFile{first[1], first[0]}
	if m2TimingTree(first) != m2TimingTree(ordered) || first[0].name != "b" {
		t.Fatal("source digest depends on enumeration order or changes its caller")
	}
	for _, changed := range [][]m2ArchiveFile{
		{{name: "b", data: []byte("changed")}, first[1]},
		{{name: "renamed", data: first[0].data}, first[1]},
		{first[0]},
	} {
		if m2TimingTree(first) == m2TimingTree(changed) {
			t.Fatal("source digest lost a file name or its bytes")
		}
	}
}

func TestM2TimingMetadataRejectsUnverifiedToolsAndSource(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"unverified", "missing-state", "missing-source", "source-command", "revision", "revision-command"} {
		s := m2MetadataSuite(t)
		m2BreakMetadataSource(t, s, mode)
		if _, err := s.timingMetadataM2(t.Context()); err == nil {
			t.Fatalf("%s: invalid metadata source passed", mode)
		}
	}
}

func m2BreakMetadataSource(t *testing.T, s *suite, mode string) {
	t.Helper()
	switch mode {
	case "unverified":
		s.m2.Ready = false
	case "missing-state":
		s.m2 = nil
	case "missing-source":
		if err := os.Remove(filepath.Join(s.root, moduleFile)); err != nil {
			t.Fatal(err)
		}
	}
	fixture := s.execute
	s.execute = func(ctx context.Context, c command) result {
		if mode == "source-command" && slices.Contains(c.args, "ls-files") {
			return result{err: ErrAcceptance}
		}
		if slices.Contains(c.args, m2RevParse) {
			if mode == "revision" {
				return result{stdout: "not an object identifier\n"}
			}
			if mode == "revision-command" {
				return result{err: ErrAcceptance}
			}
		}
		return fixture(ctx, c)
	}
}
