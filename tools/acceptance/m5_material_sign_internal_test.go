package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"

	"github.com/cristianargotti/golaw/internal/release"
)

// TestM5AuthorityRefusesWhatItCannotIssue holds the refusals of the
// material generator that an input reaches: a subject that is no URL and a
// key named on another curve than its point, which cannot sign.
func TestM5AuthorityRefusesWhatItCannotIssue(t *testing.T) {
	t.Parallel()
	a, err := m5NewAuthority(m5Moment())
	if err != nil {
		t.Fatal(err)
	}
	identity := release.Identity{Lane: "https://example.test/lane.yml", Issuer: "https://issuer.example.test"}
	if _, _, leafErr := a.leaf(identity, "%zz"); leafErr == nil {
		t.Error("a subject that is no URL issued a leaf")
	}
	valid, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	broken := *valid
	broken.Curve = elliptic.P384()
	if _, err := a.messageBundle([]byte("sums"), nil, &broken); err == nil {
		t.Error("a key that cannot sign signed checksums.txt")
	}
	if _, err := a.attestationBundle("golaw", "00", nil, &broken); err == nil {
		t.Error("a key that cannot sign signed the statement")
	}
	unsigned := *a
	unsigned.rekorKey = &broken
	if _, err := unsigned.material(nil, "hashedrekord", []byte("{}"), 1); err == nil {
		t.Error("a log key that cannot sign signed the entry")
	}
}
