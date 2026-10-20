package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/policy"
)

func TestInitRepeatPreservesEnrollmentBytesAcrossClocks(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	repo.Write("go.mod", "module example.test/service\n\ngo 1.27\n")
	ta := initBindApp(t)
	first := now()
	repeatInitAt(t, ta, repo.Root, first)
	before, record := repeatEnrollment(t, ta, repo.Root)
	if !record.WrittenAt.Equal(first) {
		t.Fatalf("first enrollment time = %s, want %s", record.WrittenAt, first)
	}
	repeatInitAt(t, ta, repo.Root, first.Add(time.Hour))
	after, repeated := repeatEnrollment(t, ta, repo.Root)
	if !bytes.Equal(before, after) {
		t.Fatalf("unchanged init rewrote enrollment bytes: WrittenAt %s became %s", record.WrittenAt, repeated.WrittenAt)
	}
}

func TestInitRepeatRewritesEnrollmentWhenPolicyBytesChange(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	repo.Write("go.mod", "module example.test/service\n\ngo 1.27\n")
	ta := initBindApp(t)
	repeatInitAt(t, ta, repo.Root, now())
	before, record := repeatEnrollment(t, ta, repo.Root)
	path := filepath.Join(repo.Root, policy.FileName)
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	repo.Write(policy.FileName, string(current)+"# Preserve the reviewed explanation on enrollment.\n")
	later := now().Add(time.Hour)
	repeatInitAt(t, ta, repo.Root, later)
	after, revised := repeatEnrollment(t, ta, repo.Root)
	if bytes.Equal(before, after) || !revised.WrittenAt.Equal(later) {
		t.Fatalf("changed policy did not refresh enrollment: WrittenAt = %s", revised.WrittenAt)
	}
	if revised.PolicyDigest != record.PolicyDigest {
		t.Fatal("comment changed the semantic policy digest")
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if revised.Bootstrap == record.Bootstrap || revised.Bootstrap != string(written) {
		t.Fatal("refreshed enrollment did not retain the changed policy bytes")
	}
}

func repeatInitAt(t *testing.T, ta testApp, root string, at time.Time) {
	t.Helper()
	ta.now = func() time.Time { return at }
	ta.stdout.Reset()
	ta.stderr.Reset()
	if code := ta.run(t.Context(), []string{"init", "--apply", root}); code != exitClean {
		t.Fatalf("init apply exit = %d: %s", code, ta.stderr.String())
	}
}

func repeatEnrollment(t *testing.T, ta testApp, root string) ([]byte, policy.Record) {
	t.Helper()
	dirs, err := ta.bind().dirs()
	if err != nil {
		t.Fatal(err)
	}
	record, found, err := policy.ReadRecord(dirs.State, root)
	if err != nil || !found {
		t.Fatalf("enrollment found = %t: %v", found, err)
	}
	data, err := os.ReadFile(policy.RecordPath(dirs.State, root))
	if err != nil {
		t.Fatal(err)
	}
	return data, record
}
