package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/release"
)

// sourceKey keys the release line of a development build, which has no
// asset of its own.
const sourceKey = "source"

// releaseDoctor is the release line of the running binary (D10, D11),
// verified under the identity a release of the kit is signed with: the
// release lane at the tag and the Actions issuer (section 4.2), bound here
// once.
func (b binding) releaseDoctor(dir, trustDir string) doctor.Line {
	return b.releaseLine(dir, trustDir, release.Identity{Lane: release.Lane, Issuer: release.DefaultIssuer})
}

// releaseLine is the release line under an identity, the seam a test
// passes its test identity through: without a directory a source build
// reports metadata and a release build names the flag that verifies it;
// with one, the verdict of release.Verify over the files the person
// downloaded and the executable's own bytes, naming the trust root a
// person gave, because a root of their own can issue any identity. A
// build that records a module sum is a go install build, a source build
// whatever files sit beside it (cut 9). Every input it cannot read is a
// red line naming it, never an error.
func (b binding) releaseLine(dir, trustDir string, id release.Identity) doctor.Line {
	version := b.pin()
	if version == "" || b.info.Sum != "" {
		return doctor.AssessRelease(sourceKey, release.Verify(release.Inputs{}))
	}
	asset := release.AssetName(version, b.goos, runtime.GOARCH)
	if dir == "" {
		return doctor.AssessRelease(asset, release.Verdict{
			State: release.StateUnknown, Code: release.CodeUnverified,
			Detail: "run golaw doctor --release <dir> with " + release.ChecksumsFile + ", its bundle and the attestation of " + asset + " to verify this binary",
		})
	}
	in, verdict, ok := b.releaseInputs(dir, trustDir, asset, release.Inputs{Version: version, Identity: id})
	if !ok {
		return doctor.AssessRelease(asset, verdict)
	}
	line := doctor.AssessRelease(asset, release.Verify(in))
	if trustDir != "" {
		line.Detail += "; under the trust root of " + trustDir
	}
	return line
}

// releaseInputs completes the version and identity of in with what
// Verify judges; a failure answers the red verdict that names it instead.
func (b binding) releaseInputs(dir, trustDir, asset string, in release.Inputs) (release.Inputs, release.Verdict, bool) {
	trust, err := trustRoot(trustDir)
	if err != nil {
		return release.Inputs{}, redVerdict(release.CodeTrustRootInvalid, err), false
	}
	in.Asset, in.Trust = asset, trust
	exe, err := b.exe()
	if err == nil {
		in.BinaryDigest, err = release.FileDigest(exe)
	}
	if err == nil {
		in.Checksums, err = readRelease(filepath.Join(dir, release.ChecksumsFile))
	}
	if err == nil {
		in.Bundle, err = readRelease(filepath.Join(dir, release.ChecksumsFile+release.BundleSuffix))
	}
	if err == nil {
		in.Attestation, err = attestation(dir, b.goos, asset, in.BinaryDigest)
	}
	if err != nil {
		return release.Inputs{}, redVerdict(release.CodeInputMissing, err), false
	}
	return in, release.Verdict{}, true
}

// trustRoot is the public good trust root the binary embeds, or the one
// of the directory --trust-root names, for a private Sigstore instance.
func trustRoot(dir string) (release.TrustRoot, error) {
	if dir == "" {
		return release.Sigstore() //nolint:wrapcheck // the trust root error names the embedded file; the line names the code
	}
	return release.LoadTrustRoot(dir) //nolint:wrapcheck // the trust root error names the file of the directory; the line names the code
}

// attestation is the first attestation file of the asset found in the
// directory; none found is no attestation, which Verify judges.
func attestation(dir, goos, asset, digest string) ([]byte, error) {
	for _, name := range attestationNames(goos, asset, digest) {
		data, err := readRelease(filepath.Join(dir, name))
		if err != nil || data != nil {
			return data, err
		}
	}
	return nil, nil
}

// attestationNames are the files an attestation of the asset is saved
// as: the bundle named after the asset, or the file gh attestation
// download writes, named after the digest with a colon, or with a dash on
// Windows, where a file name holds no colon and reading one is an error.
func attestationNames(goos, asset, digest string) []string {
	names := []string{asset + release.BundleSuffix, "sha256:" + digest + ".jsonl", "sha256-" + digest + ".jsonl"}
	if goos == "windows" {
		return slices.Delete(names, 1, 2)
	}
	return names
}

// readRelease reads one file of the release directory: an absent file is
// nil, which Verify names as missing, and any other failure is an error.
func readRelease(path string) ([]byte, error) {
	data, err := os.ReadFile(path) //nolint:gosec // a file of the release directory the person named
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("%w: read %s: %w", release.ErrInput, path, err)
	}
	return data, nil
}

func redVerdict(code string, err error) release.Verdict {
	return release.Verdict{State: release.StateRed, Code: code, Detail: err.Error()}
}

// buildLine is the line of the build metadata M1 reports.
const buildLine = "doctor.build"

// verifiedBuild leaves the build metadata line out of every report once
// the release line verified the binary: section 5.6 reports build
// metadata or release verification, and a verified binary is not the
// source build that line describes.
func verifiedBuild(reports []doctor.Report, verdict doctor.Line) []doctor.Report {
	if verdict.Code != release.CodeVerified {
		return reports
	}
	for i := range reports {
		reports[i].Lines = slices.DeleteFunc(reports[i].Lines, func(line doctor.Line) bool { return line.ID == buildLine })
	}
	return reports
}
