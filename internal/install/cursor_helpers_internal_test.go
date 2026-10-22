package install

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

type cursorEnrollment struct {
	calls                                 int
	identity, policyDigest, profileDigest string
}

func (e *cursorEnrollment) WriteEnrollment(
	_ context.Context, identity, policyDigest, profileDigest string,
) (model.Enrollment, error) {
	e.calls++
	e.identity = identity
	e.policyDigest = policyDigest
	e.profileDigest = profileDigest
	return model.Enrollment{
		State:        model.EnrollmentEnrolled,
		PolicyDigest: policyDigest, ProfileDigest: profileDigest,
	}, nil
}

func cursorOptions(t *testing.T, scope string) Options {
	t.Helper()
	home, err := filepath.EvalSymlinks(t.TempDir())
	must(t, err)
	options := Options{
		Root: filepath.Join(home, "repository"), Home: home,
		ConfigDir:  filepath.Join(home, ".cursor"),
		StateDir:   filepath.Join(home, "state"),
		Executable: filepath.Join(home, "bin área", "golaw"),
		GOOS:       runtime.GOOS, Scope: scope, Version: "0.4.0",
		PolicyDigest: "policy", ProfileDigest: "profile",
	}
	must(t, os.MkdirAll(options.Root, 0o700))
	return options
}

func cursorApply(t *testing.T, plan Plan) *cursorEnrollment {
	t.Helper()
	enrollment := &cursorEnrollment{}
	must(t, Apply(t.Context(), plan, io.Discard, enrollment))
	return enrollment
}

func cursorCheckEnrollment(
	t *testing.T, got *cursorEnrollment, options Options, calls int,
) {
	t.Helper()
	if got.calls != calls {
		t.Fatalf("enrollment calls = %d, want %d", got.calls, calls)
	}
	if calls == 0 {
		return
	}
	_, identity, err := ProjectIdentity(options.Root)
	must(t, err)
	if got.identity != identity || got.policyDigest != options.PolicyDigest ||
		got.profileDigest != options.ProfileDigest {
		t.Fatalf("enrollment = %+v, options = %+v", got, options)
	}
}

func cursorRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	must(t, err)
	return data
}

func cursorHandlers(t *testing.T, path string) map[string][]json.RawMessage {
	t.Helper()
	var document struct {
		Version int                          `json:"version"`
		Hooks   map[string][]json.RawMessage `json:"hooks"`
	}
	must(t, json.Unmarshal(cursorRead(t, path), &document))
	if document.Version != 1 {
		t.Fatalf("hooks version = %d", document.Version)
	}
	return document.Hooks
}

func cursorWriteReceipt(t *testing.T, path string, receipt Receipt) {
	t.Helper()
	data, err := json.MarshalIndent(receipt, "", "  ")
	must(t, err)
	must(t, WriteAtomic(path, append(data, '\n')))
}

func cursorInstalled(t *testing.T, scope string) (Options, Plan) {
	t.Helper()
	options := cursorOptions(t, scope)
	plan, err := PlanCursor(options)
	must(t, err)
	cursorApply(t, plan)
	return options, plan
}
