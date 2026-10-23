package setup_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/setup"
)

func TestReceiptNestedAndMalformedJSON(t *testing.T) {
	t.Parallel()
	data := receiptEdgeJSON(t, receiptEdgeValue(t))
	for _, tc := range []struct{ name, data string }{
		{"tool duplicate", strings.Replace(data, `"name":"lefthook"`, `"name":"lefthook","name":"lefthook"`, 1)},
		{"hook duplicate", strings.Replace(data, `"sha256":`, `"sha256":"`+install.Digest([]byte("hook\n"))+`","sha256":`, 1)},
		{"folded duplicate", strings.TrimSuffix(data, "}") + `,"\u017fchema":1}`},
		{"tool unknown", strings.Replace(data, `"name":"lefthook"`, `"name":"lefthook","extra":true`, 1)},
		{"hook unknown", strings.Replace(data, `"sha256":`, `"extra":true,"sha256":`, 1)},
		{"malformed", `{"schema":]`},
		{"truncated", strings.TrimSuffix(data, "}")},
		{"null", "null"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			requireReceiptDecoding(t, tc.data, true)
		})
	}
}

func TestReceiptInvalidValuesPreserveWrittenReceipt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		change func(*setup.Receipt)
	}{
		{"duplicate tool", func(r *setup.Receipt) { r.Tools = append(r.Tools, r.Tools[0]) }},
		{"duplicate hook", func(r *setup.Receipt) { r.Hooks = append(r.Hooks, r.Hooks[0]) }},
		{"odd hash", func(r *setup.Receipt) { r.Hooks[0].SHA256 = strings.Repeat("a", 63) }},
		{"short hash", func(r *setup.Receipt) { r.Hooks[0].SHA256 = strings.Repeat("a", 62) }},
		{"nonhex hash", func(r *setup.Receipt) { r.Hooks[0].SHA256 = strings.Repeat("z", 64) }},
		{"outside hook", func(r *setup.Receipt) { r.Hooks[0].Path = filepath.Join(r.Root, "pre-commit") }},
		{"nested hook", func(r *setup.Receipt) { r.Hooks[0].Path = filepath.Join(r.HooksPath, "nested", "pre-commit") }},
		{"directory hook", func(r *setup.Receipt) { r.Hooks[0].Path = r.HooksPath }},
	} {
		t.Log(tc.name)
		value := receiptEdgeValue(t)
		path := filepath.Join(t.TempDir(), "receipt.json")
		if err := setup.WriteReceipt(path, value); err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		tc.change(&value)
		if writeErr := setup.WriteReceipt(path, value); !errors.Is(writeErr, setup.ErrReceipt) {
			t.Fatalf("invalid write = %v", writeErr)
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(after, before) {
			t.Fatalf("receipt changed after invalid write: %v", err)
		}
		requireReceiptDecoding(t, receiptEdgeJSON(t, value), true)
	}
}

func TestReceiptSymlinkTargets(t *testing.T) {
	t.Parallel()
	for _, dangling := range []bool{false, true} {
		t.Run(map[bool]string{false: "existing", true: "dangling"}[dangling], func(t *testing.T) {
			t.Parallel()
			receiptEdgeSymlink(t, dangling)
		})
	}
}

func receiptEdgeSymlink(t *testing.T, dangling bool) {
	t.Helper()
	value := receiptEdgeValue(t)
	data := receiptEdgeJSON(t, value)
	target := filepath.Join(t.TempDir(), "target.json")
	if !dangling {
		if err := os.WriteFile(target, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "receipt.json")
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if _, err := setup.ReadReceipt(path); !errors.Is(err, setup.ErrReceipt) {
		t.Fatalf("symlink read = %v", err)
	}
	if err := setup.WriteReceipt(path, value); !errors.Is(err, setup.ErrReceipt) {
		t.Fatalf("symlink write = %v", err)
	}
	if got, err := os.Readlink(path); err != nil || got != target {
		t.Fatalf("symlink changed: %q, %v", got, err)
	}
	got, err := os.ReadFile(target)
	if dangling && !errors.Is(err, fs.ErrNotExist) || !dangling && (err != nil || string(got) != data) {
		t.Fatalf("symlink target changed: %v", err)
	}
}

func TestReceiptReadBounds(t *testing.T) {
	t.Parallel()
	data := receiptEdgeJSON(t, receiptEdgeValue(t))
	for _, extra := range []int{0, 1} {
		t.Run(map[int]string{0: "limit", 1: "over limit"}[extra], func(t *testing.T) {
			t.Parallel()
			padded := data + strings.Repeat(" ", model.MaxPolicyBytes-len(data)+extra)
			requireReceiptDecoding(t, padded, extra != 0)
		})
	}
}

func TestReceiptIOAndUnsupportedSchema(t *testing.T) {
	t.Parallel()
	value := receiptEdgeValue(t)
	directory := t.TempDir()
	if _, err := setup.ReadReceipt(directory); !errors.Is(err, setup.ErrReceipt) {
		t.Fatalf("directory read = %v", err)
	}
	if err := setup.WriteReceipt(directory, value); !errors.Is(err, setup.ErrReceipt) {
		t.Fatalf("directory write = %v", err)
	}
	path := filepath.Join(directory, "receipt.json")
	if err := setup.WriteReceipt(path, value); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(path, "receipt.json")
	if _, err := setup.ReadReceipt(child); err == nil {
		t.Fatal("non-directory parent read succeeded")
	}
	if err := setup.WriteReceipt(child, value); err == nil {
		t.Fatal("non-directory parent write succeeded")
	}
	value.Schema++
	if err := os.WriteFile(path, []byte(receiptEdgeJSON(t, value)), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := setup.ReadReceipt(path)
	if !errors.Is(err, setup.ErrReceipt) || !strings.Contains(err.Error(), strconv.Quote(path)) || !strings.Contains(strings.ToLower(err.Error()), "inspec") {
		t.Fatalf("unsupported schema lacks inspection guidance: %v", err)
	}
}

func receiptEdgeValue(t *testing.T) setup.Receipt {
	t.Helper()
	root, identity, err := install.ProjectIdentity(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	hooks := filepath.Join(root, "hooks")
	return setup.Receipt{
		Schema: setup.ReceiptSchema, Root: root, Identity: identity, Version: "0.2.0",
		HooksPath: hooks, Golaw: filepath.Join(root, "golaw"),
		Tools: []setup.ToolRecord{{Name: "lefthook", Version: "2.1.12", Path: filepath.Join(root, "lefthook")}},
		Hooks: []install.Artifact{{Path: filepath.Join(hooks, "pre-commit"), SHA256: install.Digest([]byte("hook\n"))}},
	}
}

func receiptEdgeJSON(t *testing.T, value setup.Receipt) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
