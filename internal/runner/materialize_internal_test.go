package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
)

type cancellingPayloadWriter struct {
	dst    io.Writer
	cancel context.CancelFunc
	wrote  *bool
}

func (w cancellingPayloadWriter) Write(data []byte) (int, error) {
	n, err := w.dst.Write(data)
	if n > 0 {
		*w.wrote = true
		w.cancel()
	}
	if err != nil {
		return n, fmt.Errorf("write cancelling payload: %w", err)
	}
	return n, nil
}

func TestMaterializeCleansInterruptedWrite(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	repo.Commit("chore: seed", map[string]string{
		"first.txt": "first\n", "second.txt": "second\n",
	})
	parent := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	wrote := false
	copyBlob := func(
		ctx context.Context, dst io.Writer, src io.Reader, size int64,
	) error {
		writer := cancellingPayloadWriter{
			dst: dst, cancel: cancel, wrote: &wrote,
		}
		return copyTreeBlob(ctx, writer, src, size)
	}
	tree, err := materialize(ctx, MaterializeInput{
		Dir: repo.Root, Env: gittest.Isolated(),
		Parent: parent, Bounds: DefaultBounds(),
	}, copyBlob)
	if !wrote || tree != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("result = %v, %v, wrote=%t", tree, err, wrote)
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatalf("resources remain = %v, %v", entries, err)
	}
}
