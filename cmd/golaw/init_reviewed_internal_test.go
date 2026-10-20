package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestInitKeepsAndEnrollsAnExistingPolicy(t *testing.T) {
	t.Parallel()
	ta, record := initApp(t)
	root := checkout(t)
	reviewed := "golaw: 1\nkit: { version: 9.9.9 }\nchecks:\n  sizes: { file_lines: 200 }\n"
	file := filepath.Join(root, bootstrap.PolicyFile)
	if err := os.WriteFile(file, []byte(reviewed), 0o600); err != nil {
		t.Fatal(err)
	}
	requirePreviewKeepsPolicy(t, ta, root, file, reviewed, record)
	ta.stdout.Reset()
	if got := ta.run(t.Context(), []string{"init", "--apply", "--language", "en-US", root}); got != exitClean {
		t.Fatalf("apply exit = %d: %s", got, ta.stderr.String())
	}
	accepted, err := os.ReadFile(file)
	if err != nil || !strings.HasPrefix(string(accepted), reviewed) ||
		!strings.Contains(string(accepted), "managed:\n") {
		t.Fatalf("reviewed policy = %q, %v", accepted, err)
	}
	if record.calls != 1 || !bytes.Equal(record.data, accepted) {
		t.Fatalf("enrollment = %+v", record)
	}
	ta.compile = func(data []byte, _ time.Time) (*model.EffectivePolicy, error) {
		if strings.Contains(string(data), "file_lines: 200") {
			return nil, errBoom
		}
		p := model.CoreDefaults()
		return &p, nil
	}
	if got := ta.run(t.Context(), []string{"init", "--apply", root}); got != exitFailure ||
		!strings.Contains(ta.stderr.String(), "boom") {
		t.Fatalf("unloadable policy: exit = %d: %s", got, ta.stderr.String())
	}
	if data, err := os.ReadFile(file); err != nil || !bytes.Equal(data, accepted) || record.calls != 1 {
		t.Fatalf("unloadable policy changed: %q, %v, %+v", data, err, record)
	}
}

// requirePreviewKeepsPolicy requires init without --apply to preview the
// templates and to leave the reviewed policy and the enrollment alone.
func requirePreviewKeepsPolicy(t *testing.T, ta testApp, root, file, reviewed string, record *enrollment) {
	t.Helper()
	if got := ta.run(t.Context(), []string{"init", root}); got != exitClean {
		t.Fatalf("preview exit = %d: %s", got, ta.stderr.String())
	}
	if !strings.Contains(ta.stdout.String(), "+++ lefthook.yml\n") {
		t.Fatalf("preview = %q", ta.stdout.String())
	}
	if data, err := os.ReadFile(file); err != nil || string(data) != reviewed || record.calls != 0 {
		t.Fatalf("preview changed policy: %q, %v, %+v", data, err, record)
	}
}
