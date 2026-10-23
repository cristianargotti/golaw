package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/release"
)

// m5Verdict is release.Verify over generated material for the running
// bytes, the files read as doctor --release reads them.
func m5Verdict(t *testing.T, m m5Material, running []byte, version string) release.Verdict {
	t.Helper()
	trust, err := release.LoadTrustRoot(m.trust)
	if err != nil {
		t.Fatal(err)
	}
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join(m.dir, name))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Fatal(err)
		}
		return data
	}
	sum := sha256.Sum256(running)
	return release.Verify(release.Inputs{
		BinaryDigest: hex.EncodeToString(sum[:]), Asset: m.asset, Version: version,
		Checksums: read(release.ChecksumsFile), Bundle: read(release.ChecksumsFile + release.BundleSuffix),
		Attestation: read(m.asset + release.BundleSuffix),
		Identity:    release.Identity{Lane: release.Lane, Issuer: release.DefaultIssuer},
		Trust:       trust, Now: m5Moment().Add(48 * time.Hour),
	})
}

func TestM5MaterialVerifiesAndEachTamperNamesItsCode(t *testing.T) {
	t.Parallel()
	running := []byte("the candidate executable\n")
	genuine := release.Identity{Lane: release.Lane, Issuer: release.DefaultIssuer}
	other := release.Identity{Lane: strings.Replace(release.Lane, "release.yml", "other.yml", 1), Issuer: release.DefaultIssuer}
	rows := []struct {
		name, tamper, code string
		signed             []byte
		identity           release.Identity
	}{
		{"near genuine", "", release.CodeVerified, running, genuine},
		{"red another binary signed", "", release.CodeChecksumMismatch, []byte("another executable\n"), genuine},
		{"red checksums", "checksums", release.CodeChecksumMismatch, running, genuine},
		{"red signature", "signature", release.CodeSignatureInvalid, running, genuine},
		{"red identity", "", release.CodeIdentityMismatch, running, other},
		{"red attestation", "attestation", release.CodeAttestationMismatch, running, genuine},
		{"red no attestation", "no-attestation", release.CodeAttestationMissing, running, genuine},
	}
	for _, row := range rows {
		m, err := m5Generate(t.TempDir(), row.signed, row.identity)
		if err != nil {
			t.Fatal(err)
		}
		if row.tamper != "" {
			if err := m5Tamper(m, row.tamper); err != nil {
				t.Fatal(err)
			}
		}
		got := m5Verdict(t, m, running, m5CandidateVersion)
		if got.Code != row.code {
			t.Errorf("%s: code %q (%s), want %q", row.name, got.Code, got.Detail, row.code)
		}
		if row.code == release.CodeVerified && (got.State != release.StateOK || got.Signer != genuine.Subject(m5CandidateVersion)) {
			t.Errorf("%s: verdict = %+v", row.name, got)
		}
	}
}

func TestM5TamperRefusesWhatItCannotRewrite(t *testing.T) {
	t.Parallel()
	if err := m5Tamper(m5Material{}, "unknown"); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("an unknown tamper = %v", err)
	}
	if err := m5Tamper(m5Material{dir: t.TempDir()}, "no-attestation"); err == nil {
		t.Fatal("an absent attestation was removed")
	}
	for _, body := range []string{"{", "{}", `{"messageSignature":{"signature":"!"}}`} {
		if _, err := m5FlipSignature([]byte(body)); err == nil {
			t.Errorf("flipped the signature of %q", body)
		}
	}
	for _, body := range []string{"{", "{}", `{"dsseEnvelope":{"payload":"!"}}`, `{"dsseEnvelope":{"payload":"e30K"}}`} {
		if _, err := m5ChangeStatement([]byte(body)); err == nil && body != `{"dsseEnvelope":{"payload":"e30K"}}` {
			t.Errorf("changed the statement of %q", body)
		}
	}
	blocked := filepath.Join(t.TempDir(), "file")
	checkedWrite(t, blocked, "a file")
	if _, err := m5Generate(blocked, []byte("x"), release.Identity{Lane: release.Lane, Issuer: release.DefaultIssuer}); err == nil {
		t.Fatal("material was written under a file")
	}
	if err := m5Rewrite(filepath.Join(t.TempDir(), "absent"), func(b []byte) ([]byte, error) { return b, nil }); err == nil {
		t.Fatal("an absent file was rewritten")
	}
}

func TestM5MaterialRefusesWhatCannotBeSigned(t *testing.T) {
	t.Parallel()
	lane := release.Identity{Lane: "https://example.test/\x7f", Issuer: release.DefaultIssuer}
	if _, err := m5Generate(t.TempDir(), []byte("x"), lane); err == nil {
		t.Fatal("a lane that is no URL was signed")
	}
	if _, err := m5DSSEBody(map[string]any{"payload": make(chan int)}, nil, nil, nil); err == nil {
		t.Fatal("an envelope that cannot be encoded was logged")
	}
	if _, err := m5JSON(make(chan int)); err == nil {
		t.Fatal("a channel was encoded")
	}
}
