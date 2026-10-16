package policy_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/policy"
)

// record is the enrollment record init --apply writes for a root.
func record(root string) policy.Record {
	return policy.Record{Root: root, PolicyDigest: "d1", ProfileDigest: "p1", KitVersion: "0.1.0", Bootstrap: minimalPolicy, WrittenBy: policy.WrittenByInit, WrittenAt: now()}
}

// writeRecord stores a record and fails the test on an error.
func writeRecord(t *testing.T, stateDir string, r policy.Record) {
	t.Helper()
	if err := policy.WriteRecord(stateDir, r); err != nil {
		t.Fatal(err)
	}
}
