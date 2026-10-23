package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"time"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/release"
)

// sourceKey keys the release line of a development build, which has no
// asset of its own.
const sourceKey = "source"

// releaseDoctor is the release line of the running binary (D10, D11):
// without a directory a source build reports metadata and a release build
// names the flag that verifies it; with one, the verdict of release.Verify
// over the files the person downloaded and the executable's own bytes.
// A build that records a module sum is a go install build the module
// proxy served, never the release lane's build in the tagged checkout,
// which records none, so it is a source build whatever files sit beside
// it (cut 9). Every input it cannot read is a red line naming it, never an
// error.
func (b binding) releaseDoctor(dir, trustDir string, now time.Time) doctor.Line {
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
	in, verdict, ok := b.releaseInputs(dir, trustDir, version, asset)
	if !ok {
		return doctor.AssessRelease(asset, verdict)
	}
	in.Now = now
	return doctor.AssessRelease(asset, release.Verify(in))
}

// releaseInputs reads what Verify judges; a failure answers the red
// verdict that names it instead.
func (b binding) releaseInputs(dir, trustDir, version, asset string) (release.Inputs, release.Verdict, bool) {
	if err := checkTrustRoot(trustDir); err != nil {
		return release.Inputs{}, redVerdict(release.CodeTrustRootInvalid, err), false
	}
	in := release.Inputs{
		Asset: asset, Version: version, Trust: release.TrustRoot{},
		Identity: release.Identity{Lane: release.Lane, Issuer: release.DefaultIssuer},
	}
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
		in.Attestation, err = attestation(dir, asset, in.BinaryDigest)
	}
	if err != nil {
		return release.Inputs{}, redVerdict(release.CodeInputMissing, err), false
	}
	return in, release.Verdict{}, true
}

// checkTrustRoot requires a given trust root to be a directory. The roots
// and the log keys are read by the verification steps that judge them, so
// until those steps exist the trust root bound is the empty one.
func checkTrustRoot(dir string) error {
	if dir == "" {
		return nil
	}
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("read the trust root %s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: the trust root %s is not a directory", model.ErrUsage, dir)
	}
	return nil
}

// attestation is the first attestation file of the asset found in the
// directory: the bundle named after the asset, or the file gh attestation
// download writes, named after the digest with a colon or, on Windows, a
// dash; none found is no attestation, which Verify judges.
func attestation(dir, asset, digest string) ([]byte, error) {
	for _, name := range []string{asset + release.BundleSuffix, "sha256:" + digest + ".jsonl", "sha256-" + digest + ".jsonl"} {
		data, err := readRelease(filepath.Join(dir, name))
		if err != nil || data != nil {
			return data, err
		}
	}
	return nil, nil
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
