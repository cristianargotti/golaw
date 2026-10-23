package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/release"
)

// The builds the release rows run: the tree and the binary built the way
// the release lane builds them, the source build without its version, and
// the tamper that names the running binary's own digest.
const (
	m5LaneTree    = "lane-tree"
	m5LaneBin     = "lane"
	m5SourceBin   = "source"
	m5RunningLine = "running-checksums"
	m5NoVCS       = "-buildvcs=false"
	m5LaneFlags   = "-ldflags=-s -w"
	m5GitClone    = "clone"
	m5GitTag      = "tag"
)

// m5LaneBinary builds the candidate the way the release lane builds it:
// go build -trimpath -ldflags=-s -w in a repository of its own that holds
// the candidate tree and the tag of the candidate version alone, so the
// toolchain stamps that version from the tag and records no module sum,
// which only a go install through the module proxy records.
func (s *suite) m5LaneBinary(ctx context.Context) (string, error) {
	tree := filepath.Join(s.base, m5LaneTree)
	clone := []string{m5GitClone, m2Quiet, "--no-tags", "--", s.root, tree}
	tag := []string{"-C", tree, m5GitTag, "v" + m5CandidateVersion}
	if err := s.gitSteps(ctx, s.root, clone, tag); err != nil {
		return "", err
	}
	binary := filepath.Join(s.base, m5LaneBin, executable(binaryName))
	args := []string{goBuild, m2Trimpath, m5LaneFlags, "-o", binary, m5CommandPackage}
	return binary, success(s.execute(ctx, command{program: s.goTool, args: args, dir: tree, env: s.env}))
}

// m5Tampered applies the tamper of a row to its material: none, the asset
// line rewritten to the digest of the binary that runs, so the checksum
// step passes and the signature over the rewritten file is what fails, or
// a tamper of m5Tamper.
func m5Tampered(m m5Material, tamper string, running []byte) error {
	switch tamper {
	case "":
		return nil
	case m5RunningLine:
		sum := sha256.Sum256(running)
		return m5Rewrite(filepath.Join(m.dir, release.ChecksumsFile), func(body []byte) ([]byte, error) {
			_, name, _ := strings.Cut(string(body), "  ")
			return []byte(hex.EncodeToString(sum[:]) + "  " + name), nil
		})
	}
	return m5Tamper(m, tamper)
}

// m5GoInstall: the candidate go install built through the module proxy
// records its module sum, so doctor reports source-build and exits 0
// with material signed over its own bytes beside it (D11, cut 9).
func (s *suite) m5GoInstall(ctx context.Context) error {
	body, err := readFile(s.binary)
	if err != nil {
		return err
	}
	m, err := m5Generate(filepath.Join(s.base, m5MaterialDir), body, m5Identity())
	if err != nil {
		return err
	}
	return s.m5JudgeSource(ctx, s.binary, m)
}

// m5SourceBuild: a go build without version control stamps no version,
// so it reports source-build whatever material sits beside it (cut 9);
// -buildvcs=false keeps a checkout at a tag, the release lane's, from
// stamping one.
func (s *suite) m5SourceBuild(ctx context.Context) error {
	binary := filepath.Join(s.base, m5SourceBin, executable(binaryName))
	args := []string{goBuild, m2Trimpath, m5NoVCS, "-o", binary, m5CommandPackage}
	if err := success(s.execute(ctx, command{program: s.goTool, args: args, dir: s.root, env: s.env})); err != nil {
		return err
	}
	body, err := readFile(binary)
	if err != nil {
		return err
	}
	m, err := m5Generate(filepath.Join(s.base, m5MaterialDir), body, m5Identity())
	if err != nil {
		return err
	}
	return s.m5JudgeSource(ctx, binary, m)
}

// m5JudgeSource requires the source line of a build: key source, code
// source-build, state unknown and exit 0.
func (s *suite) m5JudgeSource(ctx context.Context, binary string, m m5Material) error {
	line, exit, err := s.m5ReleaseDoctor(ctx, binary, m)
	if err != nil {
		return err
	}
	if line.Key != m5SourceKey || line.Code != release.CodeSourceBuild || line.State != release.StateUnknown || exit != 0 {
		return fmt.Errorf("the %s build answered %s %s %s exit %d: %w", filepath.Base(filepath.Dir(binary)), line.Key, line.State, line.Code, exit, ErrAcceptance)
	}
	return nil
}
