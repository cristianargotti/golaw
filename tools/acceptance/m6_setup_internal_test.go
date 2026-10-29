package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"runtime"
	"testing"

	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

func TestM6TamperedAssetRowRefusesWithBothDigests(t *testing.T) {
	t.Parallel()
	s := newM6Suite(m2TestSuite(t))
	if err := m6SetupCases()[0].run(s.m6Scope(m6SetupCategory, "tampered-asset"), t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestM6JudgeTamperedNamesTheExpectedAndTheObservedDigest(t *testing.T) {
	t.Parallel()
	platform := toolchain.Platform(runtime.GOOS, runtime.GOARCH)
	var asset toolchain.Asset
	for _, tool := range toolchain.ForLanguages([]surface.Language{surface.Shell}) {
		for _, candidate := range tool.Assets {
			if candidate.Platform == platform && asset.URL == "" {
				asset = candidate
			}
		}
	}
	if asset.URL == "" {
		t.Skipf("no shell tool records an asset for %s", platform)
	}
	sum := sha256.Sum256([]byte(m6Tampered))
	observed := hex.EncodeToString(sum[:])
	dir := t.TempDir()
	rows := []struct {
		name string
		err  error
		pass bool
	}{
		{"near both digests", fmt.Errorf("shellcheck: %w: expected %s, observed %s", toolchain.ErrChecksum, asset.SHA256, observed), true},
		{"red no expected digest", fmt.Errorf("shellcheck: %w: observed %s", toolchain.ErrChecksum, observed), false},
		{"red no observed digest", fmt.Errorf("shellcheck: %w: expected %s", toolchain.ErrChecksum, asset.SHA256), false},
		{"red another failure", fmt.Errorf("download: %w", ErrAcceptance), false},
		{"red installed", nil, false},
	}
	for _, row := range rows {
		if err := m6JudgeTampered(row.err, []string{asset.URL}, dir); (err == nil) != row.pass {
			t.Errorf("%s: %v", row.name, err)
		}
	}
	if err := m6JudgeTampered(rows[0].err, nil, dir); err == nil {
		t.Error("red a refusal without any fetch passed")
	}
}
