package bootstrap_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/policy"
)

func TestManagedPolicySplicesOnlyItsTwoValues(t *testing.T) {
	t.Parallel()
	in, err := (bootstrap.Inputs{
		KitVersion: "0.1.0", Language: "pt-BR",
		Layout: bootstrap.Layout{Main: bootstrap.MainRoot, Decisions: "docs/adr", Go: true},
	}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	original, err := bootstrap.Render(in)
	if err != nil {
		t.Fatal(err)
	}
	hash := install.Digest([]byte("rendered\n"))
	managed := []policy.ManagedFile{{Path: "config.txt", SHA256: hash}}
	want := strings.Replace(string(original), "version: 0.1.0", "version: 0.2.0", 1)
	want = strings.Replace(want, "managed: []\n",
		"managed:\n  - path: config.txt\n    sha256: "+hash+"\n", 1)
	got, err := bootstrap.ManagedPolicy(original, "0.2.0", managed)
	if err != nil || string(got) != want {
		t.Fatalf("policy bytes = %q, want %q, error %v", got, want, err)
	}
	again, err := bootstrap.ManagedPolicy(got, "0.2.0", managed)
	if err != nil || !bytes.Equal(got, again) {
		t.Fatalf("second splice changed bytes: %q, %v", again, err)
	}
	minimal := []byte("golaw: 1\nkit: { version: 0.1.0 } # pin\n")
	appended, err := bootstrap.ManagedPolicy(minimal, "0.1.0", managed)
	if err != nil || !bytes.HasPrefix(appended, minimal) {
		t.Fatalf("absent managed block rewrote policy: %q, %v", appended, err)
	}
}
