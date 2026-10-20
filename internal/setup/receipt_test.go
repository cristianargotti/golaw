package setup_test

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/setup"
)

func TestReceiptRoundTripAndStrictDecoding(t *testing.T) {
	t.Parallel()
	root, identity, err := install.ProjectIdentity(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	hooks := filepath.Join(root, "hooks")
	want := setup.Receipt{
		Schema: setup.ReceiptSchema, Root: root, Identity: identity, Version: "0.2.0",
		HooksPath: hooks, Golaw: filepath.Join(root, "bin", "golaw"), ArmedAt: time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC),
		Tools: []setup.ToolRecord{{
			Name: "lefthook", Version: "2.1.12",
			Path: filepath.Join(root, "bin", "lefthook"),
		}},
		Hooks: []install.Artifact{{
			Path:   filepath.Join(hooks, "pre-commit"),
			SHA256: install.Digest([]byte("hook\n")),
		}},
	}
	requireRoundTrip(t, identity, want)
	valid, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, data string
		fail       bool
	}{
		{"valid", string(valid), false},
		{"unknown", strings.TrimSuffix(string(valid), "}") + ",\"unknown\":true}", true},
		{"duplicate", strings.TrimSuffix(string(valid), "}") + ",\"schema\":1}", true},
		{"trailing", string(valid) + "\n{}", true},
		{"schema", strings.Replace(string(valid), "\"schema\":1", "\"schema\":2", 1), true},
		{"identity", strings.Replace(string(valid), identity, "", 1), true},
		{"hash", strings.Replace(string(valid), want.Hooks[0].SHA256, "invalid", 1), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			requireReceiptDecoding(t, tc.data, tc.fail)
		})
	}
}

// requireRoundTrip requires the receipt path of an identity, an absent
// receipt read as fs.ErrNotExist, and a written receipt read back whole.
func requireRoundTrip(t *testing.T, identity string, want setup.Receipt) {
	t.Helper()
	state := t.TempDir()
	path := setup.ReceiptPath(state, identity)
	if path != filepath.Join(state, "setup", install.Digest([]byte(identity))+".json") {
		t.Fatalf("receipt path = %q", path)
	}
	if _, err := setup.ReadReceipt(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing receipt = %v", err)
	}
	if err := setup.WriteReceipt(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := setup.ReadReceipt(path)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("receipt = %+v, %v", got, err)
	}
}

// requireReceiptDecoding requires a receipt file to decode, or to fail
// with ErrReceipt when fail is set.
func requireReceiptDecoding(t *testing.T, data string, fail bool) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "receipt.json")
	if err := os.WriteFile(file, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := setup.ReadReceipt(file)
	if fail && !errors.Is(err, setup.ErrReceipt) {
		t.Fatalf("expected receipt error, got %v", err)
	}
	if !fail && err != nil {
		t.Fatal(err)
	}
}
