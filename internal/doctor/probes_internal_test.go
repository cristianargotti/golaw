package doctor

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"testing"
)

func TestAgentResolutionAndIdentity(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	name := "golaw"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte("binary"), 0o700); err != nil {
		t.Fatal(err)
	}
	environment := ObserveEnvironment([]string{
		"PATH=" + directory, "PATHEXT=.EXE", "UNRELATED=value",
	}, directory)
	resolved, err := ResolveAgentBinary("golaw", environment)
	if err != nil {
		t.Fatal(err)
	}
	left, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	right, err := os.Stat(resolved)
	if err != nil || !os.SameFile(left, right) {
		t.Fatalf("resolved another file: %s %v", resolved, err)
	}
	evidence := CompareBinary("golaw", environment, path)
	if evidence.Error != "" || evidence.HookPath != evidence.RunningPath {
		t.Fatalf("identity: %#v", evidence)
	}
	if _, err := ResolveAgentBinary("golaw", AgentEnvironment{}); !errors.Is(err, ErrEnvironment) {
		t.Fatalf("shell fallback: %v", err)
	}
	if _, err := ResolveAgentBinary("missing", environment); !errors.Is(err, ErrBinary) {
		t.Fatalf("missing binary: %v", err)
	}
	if _, err := ResolveAgentBinary("../golaw", environment); !errors.Is(err, ErrBinary) {
		t.Fatalf("non-bare command: %v", err)
	}
}

func TestVersionFloorAndBuildMetadata(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{"2.1.294 (Claude Code)", false},
		{"2.1.295 (Claude Code)", true},
		{"2.1.299 (Claude Code)", true},
		{"3.0.0", true},
	} {
		got, err := VersionAtLeast(tc.value, "2.1.295")
		if err != nil || got != tc.want {
			t.Fatalf("%s: %v %v", tc.value, got, err)
		}
	}
	for _, value := range []string{"", "2.1", "2.x.295", "-1.1.295"} {
		if _, err := VersionAtLeast(value, "2.1.295"); err == nil {
			t.Fatalf("invalid version accepted: %s", value)
		}
	}
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "v0.1.0", Sum: "h1:fixture"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "revision"},
		},
	}
	got := Metadata(info)
	if got.Version != "v0.1.0" || got.Sum != "h1:fixture" || got.Revision != "revision" {
		t.Fatalf("metadata: %#v", got)
	}
	if Metadata(nil) != (BuildMetadata{}) {
		t.Fatal("missing metadata invented")
	}
	_ = RunningMetadata()
}

func TestBinaryIdentityIsTheFileNotItsName(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	name := "golaw"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	hook := filepath.Join(directory, name)
	if err := os.WriteFile(hook, []byte("binary"), 0o700); err != nil {
		t.Fatal(err)
	}
	environment := ObserveEnvironment([]string{"PATH=" + directory, "PATHEXT=.EXE"}, directory)
	other := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(other, []byte("another binary"), 0o700); err != nil {
		t.Fatal(err)
	}
	evidence := CompareBinary("golaw", environment, other)
	line := binaryLines(Snapshot{Binary: evidence})[0]
	if evidence.Error != "" || evidence.HookPath == evidence.RunningPath || line.Code != "binary-differs" {
		t.Fatalf("another executable matched the hook: %#v %#v", evidence, line)
	}
	link := filepath.Join(t.TempDir(), name)
	if err := os.Link(hook, link); err != nil {
		t.Fatal(err)
	}
	evidence = CompareBinary("golaw", environment, link)
	line = binaryLines(Snapshot{Binary: evidence})[0]
	if evidence.Error != "" || evidence.HookPath != evidence.RunningPath || line.Code != "binary-matches" {
		t.Fatalf("a hard link of the hook binary differed: %#v %#v", evidence, line)
	}
}
