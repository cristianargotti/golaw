package releasetest

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/release"
)

// Inputs reads the material as doctor reads a release directory: the hex
// SHA-256 of the binary through release.FileDigest, checksums.txt, its
// bundle, the attestation (nil once removed), the test identity and the
// trust root of the material, read by release.LoadTrustRoot as
// --trust-root reads one.
func (m Material) Inputs(tb testing.TB) release.Inputs {
	tb.Helper()
	trust, err := release.LoadTrustRoot(m.Trust)
	must(tb, err)
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
		Trust:        trust,
	}
}
