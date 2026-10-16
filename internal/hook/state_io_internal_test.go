package hook

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestPublishStateFailuresLeaveNothingBehind(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	temp, err := writeStateTemp(dir, []byte("{}"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := publishState(ctx, temp, filepath.Join(dir, "final")); !errors.Is(err, model.ErrDeadline) {
		t.Fatalf("a cancelled publication succeeded: %v", err)
	}
	if _, err := os.Stat(temp); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("the temporary survived the cancelled publication")
	}
	if err := publishState(t.Context(), filepath.Join(dir, "absent"), filepath.Join(dir, "final")); !errors.Is(err, model.ErrCannotJudge) {
		t.Fatalf("an absent temporary was published: %v", err)
	}
	if _, err := writeStateTemp(filepath.Join(dir, "missing"), []byte("{}")); !errors.Is(err, model.ErrCannotJudge) {
		t.Fatalf("a missing directory was written: %v", err)
	}
	if err := removeState(filepath.Join(dir, "absent")); err != nil {
		t.Fatal(err)
	}
	occupied := filepath.Join(dir, "occupied")
	if err := os.MkdirAll(filepath.Join(occupied, "child"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := removeState(occupied); err == nil {
		t.Fatal("a non empty directory was removed as state")
	}
}

func TestReadStateRefusesAbsentAndForgedRecords(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := readState(filepath.Join(dir, "absent.json")); !errors.Is(err, model.ErrCannotJudge) {
		t.Fatalf("an absent record was read: %v", err)
	}
	forged := filepath.Join(dir, "k.i.json")
	if err := os.WriteFile(forged, []byte(`{"key":"k","input":"i","files":{},"reason":"","hash":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readState(forged); !errors.Is(err, model.ErrCannotJudge) {
		t.Fatalf("a forged record was read: %v", err)
	}
}
