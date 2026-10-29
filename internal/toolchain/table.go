package toolchain

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/surface"
)

// Distribution is how upstream ships a tool, which decides where it lives
// in the tool directory and which runtime it needs.
type Distribution string

const (
	// DistBinary is a single executable verified by its pinned sha256.
	DistBinary Distribution = "binary"
	// DistNPM is a package of the npm lock, run on the developer's Node.
	DistNPM Distribution = "npm"
	// DistPyPI is a package of the PyPI lock, run on the developer's Python.
	DistPyPI Distribution = "pypi"
)

// The runtimes the npm and PyPI tools need, named as the table names them.
const (
	runtimeNode   = "node"
	runtimePython = "python"
)

// The archive kinds the installer can open with the standard library;
// tar.xz would need a module the kit does not carry.
const (
	archiveTarGz = "tar.gz"
	archiveZip   = "zip"
)

// Asset is one upstream download of a binary tool.
type Asset struct {
	Platform string // os-arch, as Platform answers it
	URL      string
	// SHA256 is the kit's record of the bytes, trusted on first use when
	// the pins were read (M6.12 risk 9); a download that differs is refused.
	SHA256  string
	Archive string // empty for a bare executable, tar.gz or zip
	Member  string // the executable inside the archive
}

// Tool is one pinned external linter.
type Tool struct {
	Name         string
	Version      string
	Distribution Distribution
	Languages    []surface.Language
	// Hook marks the fast tools that may run on the file just written
	// (A28); only a binary tool qualifies, because a runtime start costs
	// more than the hook budget.
	Hook   bool
	Probe  []string // the arguments that print the version
	Bin    string   // the executable an npm or PyPI package installs
	Assets []Asset
}

// Runtime is a prerequisite the developer installs: the kit never
// installs Node or Python.
type Runtime struct {
	Name  string
	Floor string
	Below string // the first version the tools refuse, empty for none
	Probe []string
}

// The embedded locks, named once because the layout, the digest and the
// installer all read them.
const (
	npmLock  = "package-lock.json"
	pypiLock = "requirements.lock"
)

// embedded carries the table and the locks inside the binary, so one kit
// version has one toolchain (A10).
//
//go:embed toolchain.yml package.json package-lock.json requirements.lock
var embedded embed.FS

// Runtime names the runtime the tool's distribution needs, empty for a
// binary.
func (t Tool) Runtime() string {
	switch t.Distribution {
	case DistNPM:
		return runtimeNode
	case DistPyPI:
		return runtimePython
	case DistBinary:
		return ""
	}
	return ""
}

func embeddedFile(name string) ([]byte, error) {
	data, err := embedded.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("embedded %s: %w", name, err)
	}
	return data, nil
}

func loadTable() (tableFile, error) {
	data, err := embeddedFile("toolchain.yml")
	if err != nil {
		return tableFile{}, fmt.Errorf("%w: %w", ErrTable, err)
	}
	return decodeTable(data)
}

// Table answers the pinned tools sorted by name.
func Table() ([]Tool, error) {
	file, err := loadTable()
	if err != nil {
		return nil, err
	}
	tools := file.tools()
	slices.SortFunc(tools, func(a, b Tool) int { return strings.Compare(a.Name, b.Name) })
	return tools, nil
}

// Lookup finds a pinned tool by its name.
func Lookup(name string) (Tool, bool) {
	tools, err := Table()
	if err != nil {
		return Tool{}, false
	}
	for i := range tools {
		if tools[i].Name == name {
			return tools[i], true
		}
	}
	return Tool{}, false
}

// ForLanguages answers, in name order, the tools that lint any of langs.
func ForLanguages(langs []surface.Language) []Tool {
	tools, err := Table()
	if err != nil {
		return nil
	}
	var out []Tool
	for i := range tools {
		if slices.ContainsFunc(tools[i].Languages, func(l surface.Language) bool { return slices.Contains(langs, l) }) {
			out = append(out, tools[i])
		}
	}
	return out
}

// Runtimes answers the two runtime prerequisites, node then python.
func Runtimes() []Runtime {
	file, err := loadTable()
	if err != nil {
		return nil
	}
	out := make([]Runtime, 0, len(file.Runtimes))
	for _, entry := range file.Runtimes {
		out = append(out, Runtime(entry))
	}
	return out
}

// Digest names the whole toolchain of this kit version: the table and
// both locks, each framed by its name and length so no two different sets
// of files hash alike. A cache keyed on it holds one toolchain.
func Digest() string {
	sum := sha256.New()
	for _, name := range []string{"toolchain.yml", npmLock, pypiLock} {
		data, err := embeddedFile(name)
		if err != nil {
			return ""
		}
		_, _ = sum.Write([]byte(name + "\x00" + strconv.Itoa(len(data)) + "\x00"))
		_, _ = sum.Write(data)
	}
	return hex.EncodeToString(sum.Sum(nil))
}

// lockIDBytes is half the digest: 16 hex digits keep the path short and
// still never collide across the few locks one machine sees.
const lockIDBytes = 8

// lockID names the directory of one lock's installation, so a new lock
// never reuses the packages of an old one.
func lockID(name string) string {
	data, err := embeddedFile(name)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:lockIDBytes])
}

// Platform joins a target the way the table names it.
func Platform(goos, goarch string) string { return goos + "-" + goarch }

func platforms() []string {
	return []string{"darwin-amd64", "darwin-arm64", "linux-amd64", "linux-arm64", "windows-amd64", "windows-arm64"}
}
