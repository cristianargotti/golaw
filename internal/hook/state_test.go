package hook_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestStateRoundTripDuplicateAndExpiry(t *testing.T) {
	t.Parallel()
	now := time.Now()
	store := hook.Store{Dir: t.TempDir(), Now: now}
	key, input := hook.Hash([]byte("key")), hook.Hash([]byte("input"))
	files := map[string]string{"a.go": hook.Hash([]byte("safe"))}
	if err := store.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(t.Context(), key, input, files, ""); err != nil {
		t.Fatal(err)
	}
	got, reason, err := store.Load(t.Context(), key)
	if err != nil || reason != "" || !maps.Equal(got, files) {
		t.Fatalf("got %v, %q, %v", got, reason, err)
	}
	if err := store.Save(t.Context(), key, input, files, ""); err == nil {
		t.Fatal("duplicate identity accepted")
	}
	store.Now = now.Add(25 * time.Hour)
	if err := store.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Load(t.Context(), key); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
}

func TestStateLoadIgnoresAnExpiredRecord(t *testing.T) {
	t.Parallel()
	now := time.Now()
	store := hook.Store{Dir: t.TempDir(), Now: now.Add(-25 * time.Hour)}
	key, input := hook.Hash([]byte("aged")), hook.Hash([]byte("input"))
	if err := store.Save(t.Context(), key, input, nil, "aged capture"); err != nil {
		t.Fatal(err)
	}
	store.Now = now
	if _, _, err := store.Load(t.Context(), key); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("a record dated 25 hours ago was loaded:", err)
	}
}

func TestStateSweepRefusesANonregularEntry(t *testing.T) {
	t.Parallel()
	store := hook.Store{Dir: t.TempDir(), Now: time.Now()}
	if err := os.Mkdir(filepath.Join(store.Dir, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := store.Sweep(t.Context()); !errors.Is(err, model.ErrCannotJudge) {
		t.Fatal("a directory in the state was swept:", err)
	}
}

func TestStateInterruptedTemporaryAndCorruption(t *testing.T) {
	t.Parallel()
	now := time.Now()
	store := hook.Store{Dir: t.TempDir(), Now: now}
	temp := filepath.Join(store.Dir, ".tmp-interrupted")
	if err := os.WriteFile(temp, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := now.Add(-2 * time.Hour)
	if err := os.Chtimes(temp, old, old); err != nil {
		t.Fatal(err)
	}
	if err := store.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(temp); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	key := hook.Hash([]byte("key"))
	corrupt := filepath.Join(store.Dir, key+".broken.json")
	if err := os.WriteFile(corrupt, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Load(t.Context(), key); err == nil {
		t.Fatal("corrupt record accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := store.Sweep(ctx); err == nil {
		t.Fatal("cancelled sweep accepted")
	}
}

func TestConcurrentStatePublication(t *testing.T) {
	t.Parallel()
	store := hook.Store{Dir: t.TempDir(), Now: time.Now()}
	key, input := hook.Hash([]byte("key")), hook.Hash([]byte("input"))
	if err := store.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var group sync.WaitGroup
	for range 2 {
		group.Go(func() {
			<-start
			results <- store.Save(t.Context(), key, input,
				map[string]string{"a.go": hook.Hash([]byte("safe"))}, "")
		})
	}
	close(start)
	group.Wait()
	close(results)
	allowed := 0
	for err := range results {
		if err == nil {
			allowed++
		}
	}
	if allowed > 1 {
		t.Fatal("two writers accepted the same invocation identity")
	}
}

func TestStateReasonAndReasonIntegrity(t *testing.T) {
	t.Parallel()
	store := hook.Store{Dir: t.TempDir(), Now: time.Now()}
	key, input := hook.Hash([]byte("reason")), hook.Hash([]byte("input"))
	if err := store.Save(t.Context(), key, input, nil, "capture unreadable"); err != nil {
		t.Fatal(err)
	}
	files, reason, err := store.Load(t.Context(), key)
	if err != nil || files != nil || reason != "capture unreadable" {
		t.Fatalf("%v, %q, %v", files, reason, err)
	}
	entries, err := os.ReadDir(store.Dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("%v, %v", entries, err)
	}
	path := filepath.Join(store.Dir, entries[0].Name())
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if decodeErr := json.Unmarshal(body, &value); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	value["reason"] = "changed reason"
	body, err = json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if writeErr := os.WriteFile(path, body, 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	if _, _, loadErr := store.Load(t.Context(), key); loadErr == nil {
		t.Fatal("changed reason accepted")
	}
}

func TestStateSweepExceedsOneBatch(t *testing.T) {
	t.Parallel()
	now := time.Now()
	store := hook.Store{Dir: t.TempDir(), Now: now.Add(-25 * time.Hour)}
	key, input := hook.Hash([]byte("expired")), hook.Hash([]byte("input"))
	if err := store.Save(t.Context(), key, input, nil, "expired capture"); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(store.Dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("%v, %v", entries, err)
	}
	expired := filepath.Join(store.Dir, entries[0].Name())
	body, err := os.ReadFile(expired)
	if err != nil {
		t.Fatal(err)
	}
	agedWrite(t, expired, body, store.Now)
	for i := range 1100 {
		agedWrite(t, filepath.Join(store.Dir, fmt.Sprintf("%s.%s.copy-%d.json", key, input, i)), body, store.Now)
		agedWrite(t, filepath.Join(store.Dir, fmt.Sprintf(".tmp-old-%d", i)), []byte("{"), store.Now)
	}
	store.Now = now
	live := hook.Hash([]byte("live"))
	if saveErr := store.Save(t.Context(), live, input, nil, "live capture"); saveErr != nil {
		t.Fatal(saveErr)
	}
	if sweepErr := store.Sweep(t.Context()); sweepErr != nil {
		t.Fatal(sweepErr)
	}
	entries, err = os.ReadDir(store.Dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("unexpected surviving entries: %d, %v", len(entries), err)
	}
	if _, reason, loadErr := store.Load(t.Context(), live); loadErr != nil || reason != "live capture" {
		t.Fatalf("%q, %v", reason, loadErr)
	}
}

// agedWrite writes a file and dates it, so a sweep sees it as old.
func agedWrite(t *testing.T, path string, body []byte, at time.Time) {
	t.Helper()
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
}

func TestStateOversizedMapBecomesReason(t *testing.T) {
	t.Parallel()
	store := hook.Store{Dir: t.TempDir(), Now: time.Now()}
	key, input := hook.Hash([]byte("large")), hook.Hash([]byte("input"))
	files := map[string]string{"large": strings.Repeat("x", (32<<20)+1)}
	if err := store.Save(t.Context(), key, input, files, ""); err != nil {
		t.Fatal(err)
	}
	got, reason, err := store.Load(t.Context(), key)
	if err != nil || got != nil || reason != "content identity record exceeds 32 MiB" {
		t.Fatalf("%v, %q, %v", got, reason, err)
	}
}
