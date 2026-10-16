package install

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

type enrollmentDouble struct {
	output   *bytes.Buffer
	calls    int
	identity string
	invalid  bool
}

func (d *enrollmentDouble) WriteEnrollment(
	_ context.Context, identity, policy, profile string,
) (model.Enrollment, error) {
	if d.calls == 0 && d.output.Len() == 0 {
		return model.Enrollment{}, model.ErrEnrollment
	}
	d.calls++
	d.identity = identity
	state := model.EnrollmentEnrolled
	if d.invalid {
		state = model.EnrollmentMismatch
	}
	return model.Enrollment{
		State: state, PolicyDigest: policy, ProfileDigest: profile,
	}, nil
}

func TestApplyDiffEnrollmentAndIdempotence(t *testing.T) {
	t.Parallel()
	options := testOptions(t)
	hook := filepath.Join(options.Root, ".git", "hooks", "pre-commit")
	must(t, os.MkdirAll(filepath.Dir(hook), 0o700))
	must(t, os.WriteFile(hook, []byte("existing\n"), 0o600))
	plan, err := PlanClaude(options)
	must(t, err)
	var output bytes.Buffer
	writer := &enrollmentDouble{output: &output}
	must(t, Apply(t.Context(), plan, &output, writer))
	if writer.identity != plan.Receipt.ProjectID {
		t.Fatal("receipt and enrollment identities differ")
	}
	before, err := os.ReadFile(plan.ReceiptPath)
	must(t, err)
	receiptHashesMatch(t, plan.ReceiptPath)
	second, err := PlanClaude(options)
	must(t, err)
	if len(second.Changes) != 0 {
		t.Fatalf("second plan contains %d changes", len(second.Changes))
	}
	output.Reset()
	must(t, Apply(t.Context(), second, &output, writer))
	after, err := os.ReadFile(plan.ReceiptPath)
	must(t, err)
	if output.Len() != 0 || !bytes.Equal(before, after) {
		t.Fatal("second installation changed output or receipt")
	}
	hookBytes, err := os.ReadFile(hook)
	must(t, err)
	if string(hookBytes) != "existing\n" {
		t.Fatal("installation changed a git hook")
	}
}

// receiptHashesMatch checks every artifact of a receipt against the bytes
// on disk.
func receiptHashesMatch(t *testing.T, path string) {
	t.Helper()
	receipt, err := ReadReceipt(path)
	must(t, err)
	for _, artifact := range receipt.Artifacts {
		data, readErr := os.ReadFile(artifact.Path)
		must(t, readErr)
		if Digest(data) != artifact.SHA256 {
			t.Fatalf("receipt hash differs: %s", artifact.Path)
		}
	}
}

func TestApplyRejectsEnrollmentAndStalePlan(t *testing.T) {
	t.Parallel()
	options := testOptions(t)
	plan, err := PlanClaude(options)
	must(t, err)
	var output bytes.Buffer
	writer := &enrollmentDouble{output: &output, invalid: true}
	if err := Apply(t.Context(), plan, &output, writer); !errors.Is(err, ErrEnrollment) {
		t.Fatalf("invalid enrollment: %v", err)
	}
	if _, err := os.Stat(plan.ReceiptPath); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("receipt written after enrollment failure: %v", err)
	}
	must(t, os.WriteFile(plan.Receipt.SettingsPath, []byte("{}\n"), 0o600))
	if err := Apply(t.Context(), plan, &output, writer); !errors.Is(err, ErrChanged) {
		t.Fatalf("stale plan: %v", err)
	}
	if err := Apply(t.Context(), Plan{}, io.Discard, nil); !errors.Is(err, ErrEnrollment) {
		t.Fatalf("missing writer: %v", err)
	}
}

func TestApplyRejectsOutputFailureAndCancellation(t *testing.T) {
	t.Parallel()
	plan, err := PlanClaude(testOptions(t))
	must(t, err)
	var output bytes.Buffer
	writer := &enrollmentDouble{output: &output}
	if err := Apply(t.Context(), plan, rejectWriter{}, writer); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("diff output failure: %v", err)
	}
	if writer.calls != 0 {
		t.Fatal("enrollment preceded a complete diff")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := Apply(ctx, plan, &output, writer); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled application: %v", err)
	}
	for _, change := range plan.Changes {
		if _, err := os.Stat(change.Path); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("artifact written after cancellation: %s: %v", change.Path, err)
		}
	}
}

func TestAtomicWriteAndDiffFailure(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "nested", "file")
	must(t, WriteAtomic(path, []byte("first")))
	must(t, WriteAtomic(path, []byte("second")))
	data, err := os.ReadFile(path)
	must(t, err)
	if string(data) != "second" {
		t.Fatal("replacement failed")
	}
	if err := WriteAtomic(filepath.Join(path, "child"), nil); err == nil {
		t.Fatal("file parent accepted")
	}
	changes := []Change{{Path: "file", After: []byte("content")}}
	if err := WriteDiff(rejectWriter{}, changes); err == nil {
		t.Fatal("diff write failure ignored")
	}
}

type rejectWriter struct{}

func (rejectWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}
