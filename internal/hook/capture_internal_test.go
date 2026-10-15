package hook

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// slowCapture is a snapshot whose every read takes a while, so a bounded
// capture meets its deadline between files.
type slowCapture struct {
	delay time.Duration
}

func (snapshot slowCapture) Files() []model.File {
	files := make([]model.File, 0, stateBatch)
	for i := range stateBatch {
		files = append(files, model.File{Path: "file-" + strings.Repeat("x", i%7) + ".go"})
	}
	return files
}

func (snapshot slowCapture) Class(string) model.FileClass {
	return model.ClassTracked
}

func (snapshot slowCapture) Read(string) ([]byte, error) {
	time.Sleep(snapshot.delay)
	return []byte("safe"), nil
}

type panickedCapture struct{}

func (panickedCapture) Files() []model.File {
	panic("controlled capture")
}

func (panickedCapture) Class(string) model.FileClass {
	return model.ClassTracked
}

func (panickedCapture) Read(string) ([]byte, error) {
	return []byte{}, nil
}

func TestCaptureFailureAndDeadline(t *testing.T) {
	t.Parallel()
	files, err := captureIdentities(t.Context(),
		proof.NewTree(map[string][]byte{"a.go": []byte("safe")}))
	if err != nil || files["a.go"] != Hash([]byte("safe")) {
		t.Fatalf("%v, %v", files, err)
	}
	if _, panicked := captureIdentities(t.Context(), panickedCapture{}); !errors.Is(panicked, model.ErrCannotJudge) || !strings.Contains(panicked.Error(), "panicked") {
		t.Fatal(panicked)
	}
	ctx, cancel := context.WithTimeout(t.Context(), captureReserve+200*time.Millisecond)
	defer cancel()
	_, late := captureIdentities(ctx, slowCapture{delay: 20 * time.Millisecond})
	if !errors.Is(late, model.ErrDeadline) || ctx.Err() != nil {
		t.Fatalf("capture consumed the answer reserve: %v, %v", late, ctx.Err())
	}
}

func TestCaptureContextRetainsAnswerReserve(t *testing.T) {
	t.Parallel()
	parent, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	child, release := captureContext(parent)
	defer release()
	parentDeadline, _ := parent.Deadline()
	childDeadline, _ := child.Deadline()
	if parentDeadline.Sub(childDeadline) != time.Second {
		t.Fatal("capture has no answer reserve")
	}
	fallback, stop := captureContext(context.WithoutCancel(t.Context()))
	defer stop()
	if _, ok := fallback.Deadline(); !ok {
		t.Fatal("capture without parent deadline is unbounded")
	}
}

func TestOptionalActionCompletionPanicAndDeadline(t *testing.T) {
	t.Parallel()
	called := false
	optionalAction(t.Context(), func(context.Context) {
		called = true
	})
	if !called {
		t.Fatal("optional action did not run")
	}
	optionalAction(t.Context(), func(context.Context) {
		panic("controlled publication")
	})
	ctx, cancel := context.WithTimeout(t.Context(), captureReserve+200*time.Millisecond)
	defer cancel()
	finished := false
	optionalAction(ctx, func(bounded context.Context) {
		<-bounded.Done()
		finished = true
	})
	if !finished || ctx.Err() != nil {
		t.Fatal("optional publication consumed the answer reserve")
	}
}

func TestSweepStateFailureAndCancellation(t *testing.T) {
	t.Parallel()
	store := Store{Dir: t.TempDir(), Now: time.Now()}
	if err := sweepState(t.Context(), store); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "occupied")
	if err := os.WriteFile(path, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	store.Dir = path
	if err := sweepState(t.Context(), store); !errors.Is(err, model.ErrCannotJudge) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	store.Dir = t.TempDir()
	if err := sweepState(ctx, store); !errors.Is(err, model.ErrDeadline) {
		t.Fatal(err)
	}
}
