// Package releasetest writes a release of the kit as the lane publishes
// it, under a temporary directory and with keys made for the test (D11):
// the binary of the running platform, checksums.txt, the Sigstore bundle
// of checksums.txt, the provenance attestation of the binary and the trust
// root that verifies them. No test reaches the network or a signing service.
package releasetest

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cristianargotti/golaw/internal/release"
)

// The identity every generated release is signed under, and the lane of
// the leaf a tampered release is signed by instead.
const (
	testLane   = "https://example.test/.github/workflows/release.yml"
	testIssuer = "https://issuer.example.test"
	otherLane  = "https://example.test/.github/workflows/other.yml"
)

// The files of the trust root directory, and the modes of what the
// helpers write under a temporary directory.
const (
	trustDir  = "trust"
	rootsFile = "roots.crt"
	rekorFile = "rekor.pub"
	dirMode   = 0o750
	fileMode  = 0o600
)

// Material is a generated release: every path is under Dir, Trust is the
// directory that holds roots.crt and rekor.pub.
type Material struct {
	Dir                                                  string
	Asset, Binary, Checksums, Bundle, Attestation, Trust string
	Identity                                             release.Identity
	Version                                              string

	keys *signer // the keys Tamper signs a piece again with
}

// Generate writes the release of version whose binary holds binary: the
// asset of runtime.GOOS and runtime.GOARCH, checksums.txt naming it, the
// bundle of checksums.txt and the attestation of the asset, both signed
// by a leaf of the test identity at refs/tags/v<version>, and the trust
// root of the certificate authority and the log that signed them.
func Generate(tb testing.TB, binary []byte, version string) Material {
	tb.Helper()
	dir := tb.TempDir()
	asset := release.AssetName(version, runtime.GOOS, runtime.GOARCH)
	m := Material{
		Dir: dir, Asset: asset, Version: version,
		Binary:      filepath.Join(dir, asset),
		Checksums:   filepath.Join(dir, release.ChecksumsFile),
		Bundle:      filepath.Join(dir, release.ChecksumsFile+release.BundleSuffix),
		Attestation: filepath.Join(dir, asset+release.BundleSuffix),
		Trust:       filepath.Join(dir, trustDir),
		Identity:    release.Identity{Lane: testLane, Issuer: testIssuer},
		keys:        newSigner(tb),
	}
	write(tb, m.Binary, binary)
	write(tb, m.Checksums, []byte(hexSum(binary)+"  "+asset+"\n"))
	m.keys.writeTrust(tb, m.Trust)
	m.writeBundle(tb, m.Identity, read(tb, m.Checksums))
	m.writeAttestation(tb, m.Identity, hexSum(binary))
	return m
}

// hexSum is the lower case hex SHA-256 of data.
func hexSum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// write writes a file and its directory.
func write(tb testing.TB, path string, data []byte) {
	tb.Helper()
	must(tb, os.MkdirAll(filepath.Dir(path), dirMode))
	must(tb, os.WriteFile(path, data, fileMode))
}

// read reads a file of the material.
func read(tb testing.TB, path string) []byte {
	tb.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // the path is a file of the material under a temporary directory
	must(tb, err)
	return data
}

// optional reads a file of the material that a tampered release may lack.
func optional(tb testing.TB, path string) []byte {
	tb.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // the path is a file of the material under a temporary directory
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	must(tb, err)
	return data
}

// must fails the test on an error of a helper.
func must(tb testing.TB, err error) {
	tb.Helper()
	if err != nil {
		tb.Fatal(err)
	}
}
