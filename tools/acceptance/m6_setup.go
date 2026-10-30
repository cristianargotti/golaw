package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// The m6-setup category and the bytes its fetcher answers for every asset.
const (
	m6SetupCategory = "m6-setup"
	m6Tampered      = "not the pinned asset\n"
)

// m6SetupCases are the m6-setup rows.
func m6SetupCases() []m6Case {
	return []m6Case{{name: "tampered-asset", run: (*m6Suite).m6TamperedAsset}}
}

// m6TamperedAsset runs the kit's own install path in process with a
// fetcher that answers every asset with other bytes: the candidate takes
// its URLs from the embedded table, and no row may reach the network for
// an asset it means to refuse. Shell needs only static binaries, so no
// runtime and no package manager runs before the first fetch.
func (s *m6Suite) m6TamperedAsset(ctx context.Context) error {
	dir := filepath.Join(s.base, "tampered-tools")
	var asked []string
	fetch := func(_ context.Context, url string) (io.ReadCloser, error) {
		asked = append(asked, url)
		return io.NopCloser(strings.NewReader(m6Tampered)), nil
	}
	_, err := toolchain.Install(ctx, toolchain.InstallOptions{
		Dir: dir, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Languages: []surface.Language{surface.Shell},
		Fetch: fetch, Run: runner.Run, Env: s.env, Out: io.Discard,
	})
	return m6JudgeTampered(err, asked, dir)
}

// m6JudgeTampered requires a checksum refusal that names the pinned digest
// of the first asset fetched and the digest of the bytes served, and a tool
// directory with no receipt and no executable of any shell tool.
func m6JudgeTampered(err error, asked []string, dir string) error {
	if len(asked) == 0 {
		return fmt.Errorf("the install fetched nothing: %w", ErrAcceptance)
	}
	if err == nil {
		return fmt.Errorf("the tampered asset was installed, want a checksum mismatch: %w", ErrAcceptance)
	}
	if !errors.Is(err, toolchain.ErrChecksum) {
		return fmt.Errorf("the tampered asset answered %w, want a checksum mismatch: %w", err, ErrAcceptance)
	}
	tools := toolchain.ForLanguages([]surface.Language{surface.Shell})
	asset, found := m6AssetOf(tools, asked[0])
	if !found {
		return fmt.Errorf("the install fetched %s, an asset of no shell tool: %w", asked[0], ErrAcceptance)
	}
	sum := sha256.Sum256([]byte(m6Tampered))
	for _, digest := range []string{asset.SHA256, hex.EncodeToString(sum[:])} {
		if !strings.Contains(err.Error(), digest) {
			return fmt.Errorf("the refusal %w does not name the digest %s: %w", err, digest, ErrAcceptance)
		}
	}
	for i := range tools {
		if err := m6NothingPlaced(dir, tools[i]); err != nil {
			return err
		}
	}
	return nil
}

// m6AssetOf is the asset of tools served at url.
func m6AssetOf(tools []toolchain.Tool, url string) (toolchain.Asset, bool) {
	for i := range tools {
		for _, asset := range tools[i].Assets {
			if asset.URL == url {
				return asset, true
			}
		}
	}
	return toolchain.Asset{}, false
}

// m6NothingPlaced requires dir to hold neither a receipt nor an executable
// of tool, so a refusal leaves nothing a later run would trust.
func m6NothingPlaced(dir string, tool toolchain.Tool) error {
	_, found, err := toolchain.ReadReceipt(dir, tool.Name)
	if err != nil {
		return fmt.Errorf("read the receipt of the refused %s: %w", tool.Name, err)
	}
	if found {
		return fmt.Errorf("the refused %s left a receipt: %w", tool.Name, ErrAcceptance)
	}
	bin := toolchain.BinPath(dir, tool, runtime.GOOS, runtime.GOARCH)
	if _, statErr := os.Stat(bin); !errors.Is(statErr, fs.ErrNotExist) {
		return fmt.Errorf("the refused %s left %s: %w", tool.Name, bin, ErrAcceptance)
	}
	return nil
}
