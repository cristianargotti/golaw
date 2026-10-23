package releasetest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/release"
)

// digestHex is the length of a hex SHA-256.
const digestHex = 64

// Tamper answers a copy of the release under a new temporary directory
// with one piece rewritten, the original left as it was: "binary" (one
// byte appended to the asset), "checksums" (the asset line names another
// digest), "signature" (the bundle signs other bytes than checksums.txt),
// "identity" (the bundle and the attestation signed by a leaf for another
// lane), "attestation" (the statement names another digest),
// "no-attestation" (the attestation file removed), "log-body" (the log
// entry of the bundle records another signature of the same leaf) and
// "attestation-log-body" (the log entry of the attestation records the
// envelope of another statement of the same leaf), each entry signed by
// the log.
func (m Material) Tamper(tb testing.TB, what string) Material {
	tb.Helper()
	out := m.moved(tb)
	binary := read(tb, out.Binary)
	switch what {
	case "binary":
		write(tb, out.Binary, append(binary, '\n'))
	case "checksums":
		write(tb, out.Checksums, []byte(strings.Repeat("0", digestHex)+"  "+out.Asset+"\n"))
	case "signature":
		out.writeBundle(tb, out.Identity, append(read(tb, out.Checksums), '\n'))
	case "identity":
		other := release.Identity{Lane: otherLane, Issuer: out.Identity.Issuer}
		out.writeBundle(tb, other, read(tb, out.Checksums))
		out.writeAttestation(tb, other, hexSum(binary))
	case "attestation":
		out.writeAttestation(tb, out.Identity, strings.Repeat("0", digestHex))
	case "no-attestation":
		must(tb, os.Remove(out.Attestation))
	case "log-body":
		checksums := read(tb, out.Checksums)
		out.writeRecordedBundle(tb, out.Identity, checksums, append(checksums, '\n'))
	case "attestation-log-body":
		out.writeRecordedAttestation(tb, out.Identity, hexSum(binary), strings.Repeat("0", digestHex))
	default:
		tb.Fatalf("releasetest: no tamper named %q", what)
	}
	return out
}

// moved copies every file of the material under a new temporary
// directory and answers the copy, so a tamper never touches the release
// another test reads.
func (m Material) moved(tb testing.TB) Material {
	tb.Helper()
	out := m
	out.Dir = tb.TempDir()
	for _, path := range []*string{&out.Binary, &out.Checksums, &out.Bundle, &out.Attestation} {
		target := filepath.Join(out.Dir, filepath.Base(*path))
		write(tb, target, read(tb, *path))
		*path = target
	}
	out.Trust = filepath.Join(out.Dir, trustDir)
	for _, name := range []string{release.RootsFile, release.RekorFile} {
		write(tb, filepath.Join(out.Trust, name), read(tb, filepath.Join(m.Trust, name)))
	}
	return out
}
