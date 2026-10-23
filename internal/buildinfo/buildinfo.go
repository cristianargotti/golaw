// Package buildinfo reads what the Go toolchain stamped into the binary and
// reports it as metadata. A source build (go install, go build) is never
// verified: the stamp authenticates nothing about the executable, so every
// consumer prints the state beside the values and the release path of
// doctor is the only verification.
package buildinfo

import (
	"regexp"
	"runtime/debug"
	"strings"
)

// DevelVersion is the version of a binary the toolchain did not version:
// a go build inside the module without a tag, or a binary without a stamp.
const DevelVersion = "0.0.0-devel"

// develStamp is what the toolchain writes for an unversioned main module.
const develStamp = "(devel)"

// releasePattern is the exact X.Y.Z form of a released version.
var releasePattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// Info is the build metadata of the running binary.
type Info struct {
	Version  string // the main module version without the leading v
	Module   string // the main module path
	Sum      string // the main module checksum, empty for a local build
	Revision string // the version control revision, empty when not stamped
	Modified bool   // the version control tree had uncommitted changes
	Stamped  bool   // the binary carries build information at all
}

// Current reads the stamp of the running binary. It is the seam newApp
// binds in production: a test cannot choose the stamp of its own binary,
// so the tests call Read with a stamp of their own.
func Current() Info {
	return Read(debug.ReadBuildInfo())
}

// Read derives the metadata from a stamp; ok false yields the unstamped
// Info with DevelVersion, so a caller never branches on a nil pointer.
func Read(info *debug.BuildInfo, ok bool) Info {
	if !ok || info == nil {
		return Info{Version: DevelVersion}
	}
	out := Info{
		Version: normalize(info.Main.Version),
		Module:  info.Main.Path,
		Sum:     info.Main.Sum,
		Stamped: true,
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			out.Revision = setting.Value
		case "vcs.modified":
			out.Modified = setting.Value == "true"
		}
	}
	return out
}

// Release answers the exact X.Y.Z version of a released binary and false for
// a development build: an unstamped binary, the toolchain placeholder or a
// pseudo-version of an untagged commit. The policy pin of A10 is checked
// against a release only.
func (i Info) Release() (string, bool) {
	if !i.Stamped || !IsRelease(i.Version) {
		return "", false
	}
	return i.Version, true
}

// IsRelease reports whether a version is the exact X.Y.Z of a release, the
// rule the pin of A10 binds, so doctor judges a recorded version the way
// scan and the hook judge the running one.
func IsRelease(version string) bool { return releasePattern.MatchString(version) }

// Lines renders the metadata one field per line for the version command:
// the version first, then the module, the sum and the revision when known,
// and the verification state last: a release version without a module sum,
// the way the release lane builds, is verified only by doctor --release,
// and any other build is never verified.
func (i Info) Lines() []string {
	lines := []string{"golaw " + i.Version}
	if i.Module != "" {
		lines = append(lines, "module: "+i.Module)
	}
	if i.Sum != "" {
		lines = append(lines, "sum: "+i.Sum)
	}
	if i.Revision != "" {
		revision := "revision: " + i.Revision
		if i.Modified {
			revision += " (modified)"
		}
		lines = append(lines, revision)
	}
	if _, release := i.Release(); release && i.Sum == "" {
		return append(lines, "build: "+ReleaseState)
	}
	return append(lines, "build: source build, executable not verifiable")
}

// ReleaseState is the verification state of a release version without a
// module sum, which only doctor --release can verify.
const ReleaseState = "release version, executable verified only by golaw doctor --release <dir>"

// normalize strips the leading v of a module version and maps the toolchain
// placeholder of an unversioned build to DevelVersion.
func normalize(version string) string {
	if version == "" || version == develStamp {
		return DevelVersion
	}
	return strings.TrimPrefix(version, "v")
}
