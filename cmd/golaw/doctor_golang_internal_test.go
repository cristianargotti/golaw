package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/gittest"
)

// goToolchainLine runs doctor --json in a repository that holds the files
// and answers its doctor.go-toolchain lines.
func goToolchainLine(t *testing.T, files map[string]string) []doctor.Line {
	t.Helper()
	repo := gittest.New(t)
	repo.Commit("chore: establish application", files)
	ta := bindApp(t)
	ta.doctorReport = ta.bind().doctorReport
	ta.run(t.Context(), []string{"doctor", "--json", repo.Root})
	decoder := json.NewDecoder(bytes.NewReader(ta.stdout.Bytes()))
	var lines []doctor.Line
	for {
		var report doctor.Report
		err := decoder.Decode(&report)
		if errors.Is(err, io.EOF) {
			return lines
		}
		if err != nil {
			t.Fatalf("decode doctor report: %v: %s", err, ta.stdout.String())
		}
		for _, line := range report.Lines {
			if line.ID == "doctor.go-toolchain" {
				lines = append(lines, line)
			}
		}
	}
}

func TestDoctorNamesTheLocalGoAgainstTheGoLine(t *testing.T) {
	t.Parallel()
	const module = "module example.test/birth\n\ngo "
	for _, tc := range []struct {
		name  string
		files map[string]string
		state string
		code  string
	}{
		{"red a go line past the local toolchain", map[string]string{"go.mod": module + "1.99.0\n"}, "older", "go-older-than-line"},
		{"red a tools module past the local toolchain", map[string]string{
			"go.mod": module + "1.24\n", "tools/go.mod": "module example.test/birth/tools\n\ngo 1.99.0\n",
		}, "older", "go-older-than-tools"},
		{"red a go.mod the go command cannot read", map[string]string{"go.mod": "module\n"}, "unknown", "go-unreadable"},
		{"near the floor of the kit", map[string]string{"go.mod": module + "1.24\n"}, "ok", "go-meets-line"},
	} {
		lines := goToolchainLine(t, tc.files)
		if len(lines) != 1 || lines[0].State != tc.state || lines[0].Code != tc.code {
			t.Fatalf("%s: lines = %+v", tc.name, lines)
		}
	}
	if lines := goToolchainLine(t, map[string]string{"README.md": "no module\n"}); len(lines) != 0 {
		t.Fatalf("a repository without go.mod has a go line: %+v", lines)
	}
}

func TestGoFactsKeepsTheLocalToolchainAndModuleBytes(t *testing.T) {
	t.Parallel()
	b, root := testBinding(t)
	files := map[string]string{
		"go.mod":       "module example.test/app\n\ngo 1.99.0\n",
		"tools/go.mod": "module\n",
	}
	writeFiles(t, root, files)
	environ := b.environ()
	b.environ = func() []string { return append(environ, "GOTOOLCHAIN=go1.99.0", "GOPROXY=off") }
	facts := b.goFacts(t.Context(), root)
	if facts.Error != "" || facts.Local == "" || facts.Line != "1.99.0" || facts.Tools != "" {
		t.Fatalf("facts = %+v", facts)
	}
	for path, want := range files {
		got, err := os.ReadFile(filepath.Join(root, path))
		if err != nil || string(got) != want {
			t.Errorf("%s changed: %q, %v", path, got, err)
		}
	}
}

func TestGoToolchainDoctorNamesProbeFailures(t *testing.T) {
	t.Parallel()
	for _, canceled := range []bool{false, true} {
		b, root := testBinding(t)
		writeFiles(t, root, map[string]string{"go.mod": "module example.test/app\n\ngo 1.24\n"})
		ctx, cancel := context.WithCancel(t.Context())
		want := "context canceled"
		if canceled {
			cancel()
		} else {
			environ := b.environ()
			b.environ = func() []string { return append(environ, "GOROOT="+filepath.Join(root, "missing")) }
			want = "cannot find GOROOT directory"
		}
		lines := b.goToolchainDoctor(ctx, root)
		cancel()
		if len(lines) != 1 || lines[0].State != "unknown" || lines[0].Code != "go-unreadable" {
			t.Fatalf("canceled %t: lines = %+v", canceled, lines)
		}
		if !strings.Contains(lines[0].Detail, "go env GOVERSION:") || !strings.Contains(lines[0].Detail, want) {
			t.Errorf("canceled %t: detail = %q", canceled, lines[0].Detail)
		}
	}
}

func TestDoctorGoDiagnosticsDoNotRefuse(t *testing.T) {
	t.Parallel()
	for _, module := range []string{"module example.test/app\n\ngo 1.99.0\n", "module\n"} {
		repo := gittest.New(t)
		repo.Commit("chore: establish application", map[string]string{"go.mod": module})
		ta := bindApp(t)
		ta.doctorReport = ta.bind().doctorReport
		if code := ta.run(t.Context(), []string{"doctor", "--json", repo.Root}); code != 0 {
			t.Fatalf("doctor exits %d: %s", code, ta.stderr.String())
		}
		if !strings.Contains(ta.stdout.String(), "doctor.go-toolchain") {
			t.Fatalf("missing Go diagnostic: %s", ta.stdout.String())
		}
	}
}
