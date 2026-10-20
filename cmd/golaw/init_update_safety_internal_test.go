package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

type updateSafetyFile struct {
	data     []byte
	modified time.Time
}

type updateSafetyOutput struct {
	action func()
	fail   bool
	seen   bool
}

func (w *updateSafetyOutput) Write(data []byte) (int, error) {
	if !w.seen && bytes.Contains(data, []byte("+++ .gitleaks.toml")) {
		w.seen = true
		if w.fail {
			return 0, io.ErrClosedPipe
		}
		w.action()
	}
	return len(data), nil
}

func TestInitUpdateCompletesOutputAndRechecksInputsBeforeWriting(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"output", "policy", "module", "template"} {
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			ta, repo, files := updateSafetyFixture(t)
			writer := &updateSafetyOutput{fail: action == "output"}
			writer.action = func() {
				path, data := ".gitleaks.toml", "title = \"concurrent edit\"\n"
				switch action {
				case "policy":
					path, data = policy.FileName, string(files[filepath.Join(repo.Root, policy.FileName)].data)+"# concurrent review\n"
				case "module":
					path, data = "go.mod", "module example.test/changed\n\ngo 1.27\n"
				case "template":
				default:
					t.Fatalf("unexpected action %q", action)
				}
				repo.Write(path, data)
				full := filepath.Join(repo.Root, path)
				files[full] = readUpdateSafetyFile(t, full)
			}
			ta.app.stdout = writer
			code := ta.run(t.Context(), []string{"init", "--update", "--apply", repo.Root})
			if code != exitFailure || !writer.seen {
				t.Fatalf("safety exit = %d, output reached = %v: %s", code, writer.seen, ta.stderr.String())
			}
			reason := "changed"
			if action == "output" {
				reason = io.ErrClosedPipe.Error()
			}
			if !strings.Contains(ta.stderr.String(), reason) {
				t.Fatalf("missing safety reason %q: %s", reason, ta.stderr.String())
			}
			requireUpdateSafetyFiles(t, files)
		})
	}
}

func TestInitUpdateCompilationFailureWritesNothing(t *testing.T) {
	t.Parallel()
	ta, repo, files := updateSafetyFixture(t)
	ta.compile = func([]byte, time.Time) (*model.EffectivePolicy, error) {
		return nil, errBoom
	}
	code := ta.run(t.Context(), []string{"init", "--update", "--apply", repo.Root})
	if code != exitFailure || !strings.Contains(ta.stderr.String(), errBoom.Error()) {
		t.Fatalf("compile failure exit = %d: %s", code, ta.stderr.String())
	}
	if ta.stdout.Len() != 0 {
		t.Fatalf("compile failure printed changes: %s", ta.stdout.String())
	}
	requireUpdateSafetyFiles(t, files)
}

func TestInitUpdateEnrollmentWeakeningWritesNothing(t *testing.T) {
	t.Parallel()
	ta, repo, files := updateSafetyFixture(t)
	stricter := strings.Replace(string(files[filepath.Join(repo.Root, policy.FileName)].data), "file_lines: 300", "file_lines: 200", 1)
	config, err := policy.Decode([]byte(stricter))
	if err != nil {
		t.Fatal(err)
	}
	dirs, err := ta.bind().dirs()
	if err != nil {
		t.Fatal(err)
	}
	record := policy.Record{
		Root: repo.Root, PolicyDigest: policy.Digest(config), KitVersion: config.Kit.Version,
		Bootstrap: stricter, WrittenBy: policy.WrittenByInit, WrittenAt: now(),
	}
	if writeErr := policy.WriteRecord(dirs.State, record); writeErr != nil {
		t.Fatal(writeErr)
	}
	path := policy.RecordPath(dirs.State, repo.Root)
	files[path] = readUpdateSafetyFile(t, path)
	code := ta.run(t.Context(), []string{"init", "--update", "--apply", repo.Root})
	if code != exitFailure || !strings.Contains(ta.stderr.String(), "weakens") {
		t.Fatalf("weakening exit = %d: %s", code, ta.stderr.String())
	}
	if ta.stdout.Len() != 0 {
		t.Fatalf("weakening printed changes: %s", ta.stdout.String())
	}
	requireUpdateSafetyFiles(t, files)
}

func updateSafetyFixture(t *testing.T) (testApp, *gittest.Repo, map[string]updateSafetyFile) {
	t.Helper()
	repo := gittest.New(t)
	repo.Write("go.mod", "module example.test/service\n\ngo 1.27\n")
	ta := initBindApp(t)
	older, config := olderPolicy(t, ta, repo, filepath.Join(repo.Root, policy.FileName))
	const previous = "title = \"previous release\"\n\n[extend]\nuseDefault = true\n"
	found := false
	for index := range config.Managed {
		if config.Managed[index].Path == ".gitleaks.toml" {
			config.Managed[index].SHA256 = install.Digest([]byte(previous))
			found = true
		}
	}
	if !found {
		t.Fatal("fixture has no managed .gitleaks.toml")
	}
	repo.Write(".gitleaks.toml", previous)
	data, err := bootstrap.ManagedPolicy([]byte(older), config.Kit.Version, config.Managed)
	if err != nil {
		t.Fatal(err)
	}
	repo.Write(policy.FileName, string(data))
	files := make(map[string]updateSafetyFile)
	for _, managed := range config.Managed {
		path := filepath.Join(repo.Root, filepath.FromSlash(managed.Path))
		files[path] = readUpdateSafetyFile(t, path)
	}
	for _, name := range []string{policy.FileName, "go.mod"} {
		path := filepath.Join(repo.Root, name)
		files[path] = readUpdateSafetyFile(t, path)
	}
	dirs, err := ta.bind().dirs()
	if err != nil {
		t.Fatal(err)
	}
	path := policy.RecordPath(dirs.State, repo.Root)
	files[path] = readUpdateSafetyFile(t, path)
	ta.stdout.Reset()
	ta.stderr.Reset()
	return ta, repo, files
}

func readUpdateSafetyFile(t *testing.T, path string) updateSafetyFile {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return updateSafetyFile{data: data, modified: info.ModTime()}
}

func requireUpdateSafetyFiles(t *testing.T, files map[string]updateSafetyFile) {
	t.Helper()
	for path, before := range files {
		after := readUpdateSafetyFile(t, path)
		if !bytes.Equal(after.data, before.data) || !after.modified.Equal(before.modified) {
			t.Errorf("refused update changed %s: %q", path, after.data)
		}
	}
}
