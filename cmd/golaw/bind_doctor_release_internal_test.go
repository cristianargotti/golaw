package main

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/release"
)

// releaseBinary is the content of the running binary the release tests
// stand for, and releaseDigest its hex SHA-256.
const releaseBinary = "golaw release binary\n"

func releaseDigest() string {
	sum := sha256.Sum256([]byte(releaseBinary))
	return hex.EncodeToString(sum[:])
}

// releaseBinding is the test binding of a build of version whose running
// executable exists and holds releaseBinary.
func releaseBinding(t *testing.T, version string) binding {
	t.Helper()
	b, home := testBinding(t)
	exe := filepath.Join(home, "bin", "golaw")
	writeFiles(t, filepath.Dir(exe), map[string]string{"golaw": releaseBinary})
	b.info.Version = version
	return b
}

// releaseDir writes the files a person downloads from a release.
func releaseDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	writeFiles(t, dir, files)
	return dir
}

func TestReleaseLineWithoutADirectory(t *testing.T) {
	t.Parallel()
	source := releaseBinding(t, "0.0.0-devel").releaseDoctor("", "")
	if source.ID != "doctor.release" || source.Key != "source" || source.State != "unknown" || source.Code != release.CodeSourceBuild {
		t.Fatalf("a source build = %+v", source)
	}
	built := releaseBinding(t, "1.2.3").releaseDoctor("", "")
	asset := release.AssetName("1.2.3", "linux", runtime.GOARCH)
	if built.Key != asset || built.State != "unknown" || built.Code != release.CodeUnverified || !strings.Contains(built.Detail, "golaw doctor --release <dir>") {
		t.Fatalf("a release build = %+v", built)
	}
}

func TestReleaseLineOverTheDownloadedFiles(t *testing.T) {
	t.Parallel()
	asset := release.AssetName("1.2.3", "linux", runtime.GOARCH)
	bundle := release.ChecksumsFile + release.BundleSuffix
	good := map[string]string{release.ChecksumsFile: releaseDigest() + "  " + asset + "\n", bundle: "{}\n"}
	other := map[string]string{release.ChecksumsFile: strings.Repeat("0", 64) + "  " + asset + "\n", bundle: "{}\n"}
	cases := map[string]struct {
		dir, trust string
		state      string
		code       string
	}{
		"a bundle that signs nothing": {releaseDir(t, good), "", "red", release.CodeSignatureInvalid},
		"checksums differ":            {releaseDir(t, other), "", "red", release.CodeChecksumMismatch},
		"no checksums":                {releaseDir(t, map[string]string{bundle: "{}\n"}), "", "red", release.CodeInputMissing},
		"no bundle":                   {releaseDir(t, map[string]string{release.ChecksumsFile: good[release.ChecksumsFile]}), "", "red", release.CodeInputMissing},
		"a trust root file":           {releaseDir(t, good), filepath.Join(releaseDir(t, good), release.ChecksumsFile), "red", release.CodeTrustRootInvalid},
		"a trust root absent":         {releaseDir(t, good), filepath.Join(t.TempDir(), "absent"), "red", release.CodeTrustRootInvalid},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			line := releaseBinding(t, "1.2.3").releaseDoctor(tc.dir, tc.trust)
			if line.Key != asset || line.State != tc.state || line.Code != tc.code {
				t.Fatalf("line = %+v", line)
			}
		})
	}
}

// A go install build records the module sum the proxy served; the
// release lane builds in the tagged checkout and records none. The first
// is a source build with or without the release files beside it (cut 9).
func TestReleaseLineCallsAGoInstallBuildASourceBuild(t *testing.T) {
	t.Parallel()
	asset := release.AssetName("1.2.3", "linux", runtime.GOARCH)
	files := map[string]string{release.ChecksumsFile: releaseDigest() + "  " + asset + "\n", release.ChecksumsFile + release.BundleSuffix: "{}\n"}
	for name, dir := range map[string]string{"without a directory": "", "with the release files": releaseDir(t, files)} {
		b := releaseBinding(t, "1.2.3")
		b.info.Sum = "h1:x"
		line := b.releaseDoctor(dir, "")
		if line.Key != "source" || line.State != "unknown" || line.Code != release.CodeSourceBuild {
			t.Errorf("%s: %+v", name, line)
		}
	}
}

func TestAVerifiedReleaseLeavesTheBuildMetadataLineOut(t *testing.T) {
	t.Parallel()
	build := doctor.Line{ID: "doctor.build", State: "source-build", Code: "source-build-unverifiable"}
	reports := func() []doctor.Report {
		return []doctor.Report{{Lines: []doctor.Line{build, {ID: "doctor.pin"}}}, {Lines: []doctor.Line{build}}}
	}
	verified := doctor.Line{ID: "doctor.release", State: "ok", Code: release.CodeVerified}
	if got := verifiedBuild(reports(), verified); len(got[0].Lines) != 1 || got[0].Lines[0].ID != "doctor.pin" || len(got[1].Lines) != 0 {
		t.Fatalf("a verified release keeps %+v", got)
	}
	for _, code := range []string{release.CodeUnverified, release.CodeSourceBuild, release.CodeChecksumMismatch} {
		if got := verifiedBuild(reports(), doctor.Line{ID: "doctor.release", Code: code}); len(got[0].Lines) != 2 || len(got[1].Lines) != 1 {
			t.Errorf("%s drops the build metadata: %+v", code, got)
		}
	}
}

func TestReleaseLineNamesAnExecutableItCannotHash(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	b.info.Version = "1.2.3"
	line := b.releaseDoctor(t.TempDir(), "")
	if line.State != "red" || line.Code != release.CodeInputMissing || !strings.Contains(line.Detail, "golaw") {
		t.Fatalf("line = %+v", line)
	}
}

func TestDoctorReleaseExitsOneOnARedLine(t *testing.T) {
	t.Parallel()
	asset := release.AssetName("1.2.3", "linux", runtime.GOARCH)
	bundle := release.ChecksumsFile + release.BundleSuffix
	cases := map[string]struct {
		version  string
		checksum string
		want     int
	}{
		"a source build": {"0.0.0-devel", releaseDigest(), exitClean},
		"tampered":       {"1.2.3", strings.Repeat("0", 64), exitFindings},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ta, _ := doctorApp(t)
			b := releaseBinding(t, tc.version)
			ta.bind = func() binding { return b }
			dir := releaseDir(t, map[string]string{release.ChecksumsFile: tc.checksum + "  " + asset + "\n", bundle: "{}\n"})
			if got := ta.run(t.Context(), []string{"doctor", "--release", dir, t.TempDir()}); got != tc.want {
				t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
			}
			if out := ta.stdout.String(); !strings.Contains(out, "doctor.release") || ta.stderr.Len() != 0 {
				t.Fatalf("stdout = %q, stderr = %q", out, ta.stderr.String())
			}
		})
	}
}
