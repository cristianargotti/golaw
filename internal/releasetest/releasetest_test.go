package releasetest_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/release"
	"github.com/cristianargotti/golaw/internal/releasetest"
)

// signedAt is the time the generator's log integrates every entry.
var signedAt = time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)

const subject = "https://example.test/.github/workflows/release.yml@refs/tags/v1.2.3"

// testBundle is the part of a Sigstore bundle these tests read back.
type testBundle struct {
	MediaType            string `json:"mediaType"`
	VerificationMaterial struct {
		Certificate struct {
			RawBytes []byte `json:"rawBytes"`
		} `json:"certificate"`
		TlogEntries []struct {
			LogIndex string `json:"logIndex"`
			LogID    struct {
				KeyID []byte `json:"keyId"`
			} `json:"logId"`
			IntegratedTime   string `json:"integratedTime"`
			InclusionPromise struct {
				SignedEntryTimestamp []byte `json:"signedEntryTimestamp"`
			} `json:"inclusionPromise"`
			CanonicalizedBody []byte `json:"canonicalizedBody"`
		} `json:"tlogEntries"`
	} `json:"verificationMaterial"`
	MessageSignature struct {
		Signature []byte `json:"signature"`
	} `json:"messageSignature"`
	DSSEEnvelope struct {
		Payload     []byte `json:"payload"`
		PayloadType string `json:"payloadType"`
		Signatures  []struct {
			Sig []byte `json:"sig"`
		} `json:"signatures"`
	} `json:"dsseEnvelope"`
}

func decode(t *testing.T, path string) testBundle {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var b testBundle
	if err := json.Unmarshal(data, &b); err != nil {
		t.Fatal(err)
	}
	return b
}

// leafOf parses the bundle's leaf and verifies it chains to the trust
// root of the material at the signing time with the code signing usage.
func leafOf(t *testing.T, b testBundle, trust release.TrustRoot) *x509.Certificate {
	t.Helper()
	cert, err := x509.ParseCertificate(b.VerificationMaterial.Certificate.RawBytes)
	if err != nil {
		t.Fatal(err)
	}
	opts := x509.VerifyOptions{Roots: trust.Roots, CurrentTime: signedAt, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning}}
	if _, err := cert.Verify(opts); err != nil {
		t.Fatalf("the leaf chains to the trust root: %v", err)
	}
	return cert
}

// requireLogged verifies the signed entry timestamp of the one log entry
// under the log key of the trust root.
func requireLogged(t *testing.T, b testBundle, trust release.TrustRoot) {
	t.Helper()
	if len(b.VerificationMaterial.TlogEntries) != 1 {
		t.Fatalf("entries = %d", len(b.VerificationMaterial.TlogEntries))
	}
	e := b.VerificationMaterial.TlogEntries[0]
	id := hex.EncodeToString(e.LogID.KeyID)
	key, ok := trust.Rekor[id].(*ecdsa.PublicKey)
	if !ok || e.IntegratedTime != strconv.FormatInt(signedAt.Unix(), 10) {
		t.Fatalf("log %s, integrated %s", id, e.IntegratedTime)
	}
	payload := fmt.Sprintf(`{"body":%q,"integratedTime":%s,"logID":%q,"logIndex":%s}`,
		base64.StdEncoding.EncodeToString(e.CanonicalizedBody), e.IntegratedTime, id, e.LogIndex)
	sum := sha256.Sum256([]byte(payload))
	if !ecdsa.VerifyASN1(key, sum[:], e.InclusionPromise.SignedEntryTimestamp) {
		t.Fatal("the signed entry timestamp verifies under the log key")
	}
}

// identityOf is the subject alternative name and the two issuer
// extensions of a leaf.
func identityOf(t *testing.T, cert *x509.Certificate) (string, string, string) {
	t.Helper()
	var v1, v2 string
	for _, ext := range cert.Extensions {
		switch ext.Id.String() {
		case "1.3.6.1.4.1.57264.1.1":
			v1 = string(ext.Value)
		case "1.3.6.1.4.1.57264.1.8":
			if _, err := asn1.UnmarshalWithParams(ext.Value, &v2, "utf8"); err != nil {
				t.Fatal(err)
			}
		}
	}
	if len(cert.URIs) != 1 {
		t.Fatalf("URIs = %v", cert.URIs)
	}
	return cert.URIs[0].String(), v1, v2
}

func TestGenerateWritesTheReleaseOfThePlatform(t *testing.T) {
	t.Parallel()
	m := releasetest.Generate(t, []byte("binary\n"), "1.2.3")
	asset := release.AssetName("1.2.3", runtime.GOOS, runtime.GOARCH)
	sum := sha256.Sum256([]byte("binary\n"))
	in := m.Inputs(t)
	if m.Asset != asset || m.Binary != filepath.Join(m.Dir, asset) || m.Version != "1.2.3" || m.Identity.Subject("1.2.3") != subject {
		t.Fatalf("material = %+v", m)
	}
	if got, ok := release.ChecksumOf(in.Checksums, asset); !ok || got != hex.EncodeToString(sum[:]) || in.BinaryDigest != got {
		t.Fatalf("checksums.txt = %q, digest %s", in.Checksums, in.BinaryDigest)
	}
	if m.Attestation != filepath.Join(m.Dir, asset+release.BundleSuffix) || len(in.Attestation) == 0 || in.Identity.Issuer != "https://issuer.example.test" {
		t.Fatalf("inputs = %+v", in)
	}
	for _, name := range []string{"roots.crt", "rekor.pub"} {
		if _, err := os.Stat(filepath.Join(m.Trust, name)); err != nil {
			t.Errorf("the trust root holds %s: %v", name, err)
		}
	}
}

func TestTheBundleSignsChecksumsUnderTheTestIdentity(t *testing.T) {
	t.Parallel()
	m := releasetest.Generate(t, []byte("binary\n"), "1.2.3")
	in := m.Inputs(t)
	b := decode(t, m.Bundle)
	if b.MediaType != "application/vnd.dev.sigstore.bundle.v0.3+json" {
		t.Fatalf("media type %s", b.MediaType)
	}
	cert := leafOf(t, b, in.Trust)
	if san, v1, v2 := identityOf(t, cert); san != subject || v1 != in.Identity.Issuer || v2 != in.Identity.Issuer {
		t.Fatalf("identity %s, %s, %s", san, v1, v2)
	}
	sum := sha256.Sum256(in.Checksums)
	key, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok || !ecdsa.VerifyASN1(key, sum[:], b.MessageSignature.Signature) {
		t.Fatal("the signature verifies over checksums.txt")
	}
	requireLogged(t, b, in.Trust)
}

func TestTheAttestationNamesTheAssetAtItsDigest(t *testing.T) {
	t.Parallel()
	m := releasetest.Generate(t, []byte("binary\n"), "1.2.3")
	in := m.Inputs(t)
	b := decode(t, m.Attestation)
	cert := leafOf(t, b, in.Trust)
	env := b.DSSEEnvelope
	pae := fmt.Sprintf("DSSEv1 %d %s %d %s", len(env.PayloadType), env.PayloadType, len(env.Payload), env.Payload)
	sum := sha256.Sum256([]byte(pae))
	key, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if env.PayloadType != "application/vnd.in-toto+json" || len(env.Signatures) != 1 || !ok || !ecdsa.VerifyASN1(key, sum[:], env.Signatures[0].Sig) {
		t.Fatal("the envelope verifies under the leaf")
	}
	want := fmt.Sprintf(`"subject":[{"digest":{"sha256":%q},"name":%q}]`, in.BinaryDigest, in.Asset)
	if !strings.Contains(string(env.Payload), want) {
		t.Fatalf("statement %s", env.Payload)
	}
	if san, _, _ := identityOf(t, cert); san != subject {
		t.Fatalf("identity %s", san)
	}
	requireLogged(t, b, in.Trust)
}

// tamperCheck holds what one tamper must have changed in its copy.
type tamperCheck func(t *testing.T, good release.Inputs, got releasetest.Material, in release.Inputs)

func binaryChanged(t *testing.T, good release.Inputs, _ releasetest.Material, in release.Inputs) {
	t.Helper()
	if in.BinaryDigest == good.BinaryDigest || !bytes.Equal(in.Checksums, good.Checksums) {
		t.Error("the binary changes, checksums.txt does not")
	}
}

func checksumsChanged(t *testing.T, good release.Inputs, _ releasetest.Material, in release.Inputs) {
	t.Helper()
	if got, ok := release.ChecksumOf(in.Checksums, in.Asset); !ok || got == good.BinaryDigest || in.BinaryDigest != good.BinaryDigest {
		t.Errorf("the asset line names %s", got)
	}
}

func signatureChanged(t *testing.T, _ release.Inputs, got releasetest.Material, in release.Inputs) {
	t.Helper()
	b := decode(t, got.Bundle)
	key, ok := leafOf(t, b, in.Trust).PublicKey.(*ecdsa.PublicKey)
	sum := sha256.Sum256(in.Checksums)
	if !ok || ecdsa.VerifyASN1(key, sum[:], b.MessageSignature.Signature) {
		t.Error("the signature no longer verifies over checksums.txt")
	}
}

func identityChanged(t *testing.T, _ release.Inputs, got releasetest.Material, in release.Inputs) {
	t.Helper()
	for _, path := range []string{got.Bundle, got.Attestation} {
		if san, _, _ := identityOf(t, leafOf(t, decode(t, path), in.Trust)); san != "https://example.test/.github/workflows/other.yml@refs/tags/v1.2.3" {
			t.Errorf("%s signed by %s", path, san)
		}
	}
}

func attestationChanged(t *testing.T, _ release.Inputs, got releasetest.Material, in release.Inputs) {
	t.Helper()
	if payload := decode(t, got.Attestation).DSSEEnvelope.Payload; strings.Contains(string(payload), in.BinaryDigest) {
		t.Error("the statement names another digest")
	}
}

func attestationRemoved(t *testing.T, _ release.Inputs, _ releasetest.Material, in release.Inputs) {
	t.Helper()
	if in.Attestation != nil {
		t.Error("the attestation is gone")
	}
}

// logBodyChanged requires the log entry of the bundle, signed by the log,
// to record another signature than the bundle's own.
func logBodyChanged(t *testing.T, _ release.Inputs, got releasetest.Material, in release.Inputs) {
	t.Helper()
	b := decode(t, got.Bundle)
	requireLogged(t, b, in.Trust)
	if bytes.Contains(b.VerificationMaterial.TlogEntries[0].CanonicalizedBody, []byte(base64.StdEncoding.EncodeToString(b.MessageSignature.Signature))) {
		t.Error("the log entry records another signature than the bundle's")
	}
}

// attestationLogBodyChanged requires the log entry of the attestation,
// signed by the log, to record another envelope than the attestation's.
func attestationLogBodyChanged(t *testing.T, _ release.Inputs, got releasetest.Material, in release.Inputs) {
	t.Helper()
	b := decode(t, got.Attestation)
	requireLogged(t, b, in.Trust)
	if bytes.Contains(b.VerificationMaterial.TlogEntries[0].CanonicalizedBody, []byte(base64.StdEncoding.EncodeToString(b.DSSEEnvelope.Signatures[0].Sig))) {
		t.Error("the log entry records another envelope than the attestation's")
	}
}

func TestTamperRewritesOnePieceOfACopy(t *testing.T) {
	t.Parallel()
	m := releasetest.Generate(t, []byte("binary\n"), "1.2.3")
	good := m.Inputs(t)
	cases := map[string]tamperCheck{
		"binary": binaryChanged, "checksums": checksumsChanged, "signature": signatureChanged,
		"identity": identityChanged, "attestation": attestationChanged, "no-attestation": attestationRemoved,
		"log-body": logBodyChanged, "attestation-log-body": attestationLogBodyChanged,
	}
	for what, check := range cases {
		t.Run(what, func(t *testing.T) {
			t.Parallel()
			got := m.Tamper(t, what)
			if got.Dir == m.Dir || filepath.Dir(got.Binary) != got.Dir || filepath.Dir(got.Trust) != got.Dir {
				t.Fatalf("the tamper writes a copy: %+v", got)
			}
			check(t, good, got, got.Inputs(t))
			again := m.Inputs(t)
			if again.BinaryDigest != good.BinaryDigest || !bytes.Equal(again.Bundle, good.Bundle) || !bytes.Equal(again.Attestation, good.Attestation) {
				t.Error("the original is untouched")
			}
		})
	}
}
