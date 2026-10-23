package releasetest

import (
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/release"
)

// Inputs reads the material as doctor reads a release directory: the hex
// SHA-256 of the binary through release.FileDigest, checksums.txt, its
// bundle, the attestation (nil once removed), the test identity and the
// trust root of the material.
func (m Material) Inputs(tb testing.TB) release.Inputs {
	tb.Helper()
	digest, err := release.FileDigest(m.Binary)
	must(tb, err)
	return release.Inputs{
		BinaryDigest: digest,
		Asset:        m.Asset,
		Version:      m.Version,
		Checksums:    read(tb, m.Checksums),
		Bundle:       read(tb, m.Bundle),
		Attestation:  optional(tb, m.Attestation),
		Identity:     m.Identity,
		Trust:        trustRoot(tb, m.Trust),
	}
}

// trustRoot reads roots.crt and rekor.pub of a trust directory: every
// certificate a root, every public key a log keyed by its id, the hex
// SHA-256 of its DER form.
func trustRoot(tb testing.TB, dir string) release.TrustRoot {
	tb.Helper()
	trust := release.TrustRoot{Roots: x509.NewCertPool(), Rekor: map[string]crypto.PublicKey{}}
	for _, der := range blocks(read(tb, filepath.Join(dir, rootsFile))) {
		cert, err := x509.ParseCertificate(der)
		must(tb, err)
		trust.Roots.AddCert(cert)
	}
	for _, der := range blocks(read(tb, filepath.Join(dir, rekorFile))) {
		key, err := x509.ParsePKIXPublicKey(der)
		must(tb, err)
		id := sha256.Sum256(der)
		trust.Rekor[hex.EncodeToString(id[:])] = key
	}
	return trust
}

// blocks are the bytes of every PEM block of data.
func blocks(data []byte) [][]byte {
	var out [][]byte
	for block, rest := pem.Decode(data); block != nil; block, rest = pem.Decode(rest) {
		out = append(out, block.Bytes)
	}
	return out
}
